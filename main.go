package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"os/exec"

	"github.com/cheggaaa/pb/v3"
	"github.com/sofuetakuma112/udmey-caps-translater/file"
	"github.com/sofuetakuma112/udmey-caps-translater/firebase"
	"github.com/sofuetakuma112/udmey-caps-translater/translate"
	"github.com/sofuetakuma112/udmey-caps-translater/types"
)

type Caption struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"subtitle"`
}

type Captions []*Caption

type WordWithTimeStamp struct {
	Word      string  `json:"word"`
	Timestamp float64 `json:"timestamp"`
}

type WordDict []*WordWithTimeStamp

type WordGroup []WordDict

// 抽出した字幕データを整形する
func formatCaptions(outputDirPath string, rawCaptions Captions) (Captions, error) {
	var formattedCaps Captions

	path := outputDirPath + "/formattedCaptions.json"
	if file.CheckFileExist(path) {
		readBytes, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		json.Unmarshal(readBytes, &formattedCaps)
		return formattedCaps, nil
	}

	videoDuration := rawCaptions[len(rawCaptions)-1].To

	var formattedWords []string
	for i, c := range rawCaptions {
		// 字幕テキストの処理
		idx := strings.Index(c.Text, ".")
		newText := c.Text
		// 文末のピリオドを取り除く
		if idx != -1 {
			if idx == len(c.Text)-1 { // 末尾がピリオド
				newText = c.Text[:idx]
			} else if idx > 0 && (string(c.Text[idx-1]) == " " || string(c.Text[idx+1]) == " ") { // "aa. aa" or "aa .aa"のケース
				newText = c.Text[0:idx] + c.Text[idx+1:]
			}
		}

		// 文字列のすべてのカンマを空文字に置き換える
		newText = strings.ReplaceAll(newText, ",", "")

		// ’ => 'に置換
		newText = strings.ReplaceAll(newText, "’", "'")

		// ! => に置換
		newText = strings.ReplaceAll(newText, "!", "")

		// “ => "に置換
		newText = strings.ReplaceAll(newText, "“", "\"")

		// ” => "に置換
		newText = strings.ReplaceAll(newText, "”", "\"")

		re := regexp.MustCompile(`\s+`)
		words := re.Split(newText, -1)

		caption := &Caption{
			From: c.From,
			To:   videoDuration,
			Text: strings.Join(words, " "),
		}

		if len(rawCaptions)-1 == i {
			caption.To = videoDuration
		} else {
			caption.To = rawCaptions[i+1].From
		}

		formattedWords = append(formattedWords, words...)
		formattedCaps = append(formattedCaps, caption)
	}

	formattedText := strings.ToLower(strings.Join(formattedWords, " "))
	err := ioutil.WriteFile(outputDirPath+"/"+escapedPuncTxtName, []byte(formattedText), 0644)
	if err != nil {
		log.Fatal(err)
	}

	file, _ := json.MarshalIndent(formattedCaps, "", " ")
	err = ioutil.WriteFile(path, file, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return formattedCaps, nil
}

func createDict(outputDirPath string, captions Captions) WordDict {
	var dict WordDict
	path := outputDirPath + "/dict.json"

	if file.CheckFileExist(path) {
		readBytes, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		json.Unmarshal(readBytes, &dict)
		return dict
	}

	for _, c := range captions {
		words := strings.Split(c.Text, " ")
		countOfWords := len(words)

		from_float := likeIso2Float(c.From)
		to_float := likeIso2Float(c.To)

		lenOfTalk := to_float - from_float

		var secOfBetWords float64 = 0
		if countOfWords != 1 {
			secOfBetWords = lenOfTalk / float64(countOfWords-1)
		}

		for i, w := range words {
			dict = append(dict, &WordWithTimeStamp{
				Word:      w,
				Timestamp: from_float + float64(i)*secOfBetWords,
			})
		}
	}

	file, _ := json.MarshalIndent(dict, "", " ")
	_ = ioutil.WriteFile(path, file, 0644)

	return dict
}

func groupBySentence(outputDirPath string, puncRestoredText string, dict WordDict) types.Sentences {
	var wordsBySentence WordDict
	var sentences types.Sentences

	path := outputDirPath + "/captions_en_by_sentence.json"

	if file.CheckFileExist(path) {
		readBytes, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		json.Unmarshal(readBytes, &sentences)
		return sentences
	}

	restoredWords := strings.Split(puncRestoredText, " ")
	for i, rw := range restoredWords {
		dictWord := dict[0].Word
		timestamp := dict[0].Timestamp

		if strings.Index(strings.ToLower(rw), strings.ToLower(dictWord)) != -1 {
			indexOfLastChar := len(rw) - 1

			hasPunc := false
			for _, punc := range []string{".", "?"} {
				if strings.Index(rw, punc) == indexOfLastChar { // 末尾文字が句読点
					hasPunc = true
				}
			}
			// 次の単語が文章の先頭に来る単語なら、現在の単語を文章の末尾単語とする
			isLastWord := false
			for _, firstWord := range []string{"It"} {
				if len(restoredWords)-1 != i && firstWord == restoredWords[i+1] { // 次の単語が文章の先頭に来る単語の場合
					isLastWord = true
				}
			}

			wordsBySentence = append(wordsBySentence, &WordWithTimeStamp{
				Word:      rw,
				Timestamp: timestamp,
			})

			if hasPunc || isLastWord { // 直前でappendしたWordWithTimeStampのWordに文末記号が含まれていた
				// wordsBySentenceを{ from, to, sentence }の形状に変換する
				var words []string
				for _, w := range wordsBySentence {
					words = append(words, w.Word)
				}
				sentence := types.Sentence{
					From:     ms2likeISOFormat(int(wordsBySentence[0].Timestamp * 1000))[3:],
					To:       ms2likeISOFormat(int(wordsBySentence[len(wordsBySentence)-1].Timestamp * 1000))[3:],
					Sentence: unescapeDot(strings.Join(words, " ")),
				}

				// FIXME: 句読点以外に対応する必要があるかも
				if isLastWord && string(sentence.Sentence[len(sentence.Sentence)-1]) != "." { // 次の単語が先頭単語でかつ現在の文章の末尾に句読点が存在しない
					sentence.Sentence += "."
				}

				sentences = append(sentences, sentence)
				wordsBySentence = nil
			}
		} else {
			log.Fatal(fmt.Errorf("strings.ToLower(rw): %v, strings.ToLower(dictWord): %v, timestamp: %v", strings.ToLower(rw), strings.ToLower(dictWord), timestamp))
		}

		dict = dict[1:]
	}

	file, _ := json.MarshalIndent(sentences, "", " ")
	_ = ioutil.WriteFile(path, file, 0644)

	return sentences
}

func translateSentences(outputDirPath string, sentences types.Sentences, apiUrlFormat string) types.Sentences {
	var jpSentences types.Sentences

	path := outputDirPath + "/captions_ja_by_sentence.json"
	tmpPath := outputDirPath + "/captions_ja_by_sentence_tmp.json"

	// 翻訳済みならそのファイルをreadして返す
	if file.CheckFileExist(path) {
		readBytes, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		json.Unmarshal(readBytes, &jpSentences)
		return jpSentences
	}

	fmt.Println("翻訳途中なら一時保存ファイルをreadする")

	// 翻訳途中なら一時保存ファイルをreadする
	jpSentences = make(types.Sentences, len(sentences))
	translatedSentenceIdxes := []int{}
	if file.CheckFileExist(tmpPath) {
		readBytes, err := ioutil.ReadFile(tmpPath)
		if err != nil {
			panic(err)
		}

		json.Unmarshal(readBytes, &jpSentences)

		for i, tjs := range jpSentences {
			notSame := tjs != types.Sentence{}
			if notSame { // 初期値と一致しない
				// 和訳済みのSentenceのidxを集める
				translatedSentenceIdxes = append(translatedSentenceIdxes, i)
			}
		}
	}

	// innerBar := pb.StartNew(len(sentences))

	var wg sync.WaitGroup
	var mu sync.Mutex
	// jpSentences = make(Sentences, len(sentences))
	semaphore := make(chan struct{}, 10)

	containsValue := func(s []int, val int) bool {
		for _, v := range s {
			if v == val {
				return true
			}
		}
		return false
	}

	for i, s := range sentences {
		isContain := containsValue(translatedSentenceIdxes, i)
		if isContain {
			// 既に翻訳済みなのでスキップ
			continue
		}

		semaphore <- struct{}{}
		wg.Add(1)
		go func(i int, s types.Sentence) {
			defer func() {
				<-semaphore
				// innerBar.Increment()
				wg.Done()
			}()
			translatedText, err := translate.Translate(s.Sentence, apiUrlFormat)
			if err != nil {
				// 途中の状態をJSONにダンプする
				file, _ := json.MarshalIndent(jpSentences, "", " ")
				err = ioutil.WriteFile(tmpPath, file, 0644)
				if err != nil {
					log.Fatal(fmt.Errorf("現在の状態をJSONにダンプするのに失敗: %w", err))
				}
				log.Fatal(fmt.Errorf("failed with %q: %w", outputDirPath, err))
			}

			jpSentence := types.Sentence{
				Sentence: translatedText.Text,
				From:     s.From,
				To:       s.To,
			}
			mu.Lock()
			jpSentences[i] = jpSentence
			mu.Unlock()
		}(i, s)
	}
	wg.Wait()
	// innerBar.Finish()

	file, _ := json.MarshalIndent(jpSentences, "", " ")
	_ = ioutil.WriteFile(path, file, 0644)

	return jpSentences
}

func createSrt(outputDirPath string, jpSentences types.Sentences) {
	srt := ""

	path := outputDirPath + "/captions_ja.srt"
	if file.CheckFileExist(path) {
		return
	}

	for i, js := range jpSentences {
		jpText := js.Sentence
		from := js.From
		to := js.To

		srt += fmt.Sprintf("%v\n%v --> %v\n%v\n\n", i+1, strings.Replace(from, ".", ",", 1), strings.Replace(to, ".", ",", 1), jpText)
	}
	_ = ioutil.WriteFile(path, []byte(srt), 0644)
}

func repunc(outputDirPath string, courceId, lectureId string) string {
	puncRestoredTextFilePath := outputDirPath + "/" + restoredPuncTxtName
	if !file.CheckFileExist(puncRestoredTextFilePath) {
		err := exec.Command("python3", "repunc_by_nemo.py", courceId, lectureId, escapedPuncTxtName, restoredPuncTxtName).Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	readBytes, err := ioutil.ReadFile(puncRestoredTextFilePath)
	if err != nil {
		panic(err)
	}
	return string(readBytes)
}

const escapedPuncTxtName string = "formatted_captions.txt"
const restoredPuncTxtName string = "textPuncEscapedAndRestored.txt"

// func init() {
// 	flag.Parse()
// }

func main() {
	rootDir := "./jsons" // ディレクトリのパスを指定
	var jsonFiles []string

	// Walk関数を使って、rootDir内のすべてのファイルを走査する
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			jsonFiles = append(jsonFiles, path) // JSONファイルの場合、パスをスライスに追加
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	type TextOutput struct {
		path      string
		courceId  string
		lectureId string
		sentences types.Sentences
	}

	textOutputs := []*TextOutput{}

	for i, jsonFilePath := range jsonFiles {
		fmt.Printf("%d / %d\n", i+1, len(jsonFiles))

		jsonFilePath := jsonFilePath
		var rawCaptions []*Caption

		file, err := os.Open(jsonFilePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		byteValue, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(byteValue, &rawCaptions)
		if err != nil {
			log.Fatal(err)
		}

		if len(rawCaptions) == 0 {
			continue
		}

		filename := filepath.Base(jsonFilePath)
		extension := filepath.Ext(filename)
		name := filename[0 : len(filename)-len(extension)]

		// 正規表現パターンを定義
		pattern := regexp.MustCompile(`(\d+)-(\d+)`)

		// 正規表現パターンにマッチする部分を検索
		matches := pattern.FindStringSubmatch(name)

		courceId := ""
		lectureId := ""

		// マッチした部分文字列を取得
		if len(matches) > 0 {
			courceId = matches[1]
			lectureId = matches[2]
		}

		if courceId == "" || lectureId == "" {
			log.Fatal(errors.New("コースIDもしくはレクチャーIDが不正"))
		}

		crrDir, _ := os.Getwd()
		outputDirPath := crrDir + "/captions" + "/" + courceId + "/" + lectureId
		if err := os.MkdirAll(outputDirPath, 0777); err != nil {
			log.Fatal(err)
		}

		fmt.Println(outputDirPath)

		captions, err := formatCaptions(outputDirPath, rawCaptions)
		if err != nil {
			log.Fatal(err)
		}
		dict := createDict(outputDirPath, captions)
		puncRestoredText := repunc(outputDirPath, courceId, lectureId)
		sentences := groupBySentence(outputDirPath, puncRestoredText, dict)

		textOutputs = append(textOutputs, &TextOutput{
			path:      outputDirPath,
			courceId:  courceId,
			lectureId: lectureId,
			sentences: sentences,
		})
	}

	// APIのエンドポイントと1動画の字幕データを1:1で対応付けて並列で和訳する
	availableUrls := []string{"https://script.google.com/macros/s/AKfycbwU3rp-wP0wC0rHy1uajb61bKCQGDB4TJ8HofbtU_KCB3hmjKol0-_I8ABXr9Pr_aIAOg/exec?text=%v&source=en&target=ja", "https://script.google.com/macros/s/AKfycbxXtSoPH_UDtGD-bZpWt6Gx2m3s0GyKTjO1LHCteVvMJNje5PDytKmzzTR7vRMb0Nmm/exec?text=%v&source=en&target=ja", "https://script.google.com/macros/s/AKfycbyQAvp99EoatfQYZ3pBQDpLr4TWazEUzyNFAiNUT3osWD388S27hHaPx0sjuNe7nZON0A/exec?text=%v&source=en&target=ja", "https://script.google.com/macros/s/AKfycbwPd2RT9cOHksOSodK9R-ERoqGWgwBntLFOKhZtMEk5AcAlI6J0uCOlJ2gCcxQ9MhpKrA/exec?text=%v&source=en&target=ja%22%7D"}

	usingUrls := make([]bool, len(availableUrls))
	for i := range usingUrls {
		usingUrls[i] = false
	}

	semaphore := make(chan struct{}, len(availableUrls))

	outerBar := pb.StartNew(len(textOutputs))

	var wg sync.WaitGroup

	for _, to := range textOutputs {
		to := to
		outputDirPath := to.path
		sentences := to.sentences
		wg.Add(1)

		semaphore <- struct{}{}
		// 使用可能なURLが最低一つはある
		urlIdx := getUnusedUrlIdx(availableUrls, usingUrls)
		if urlIdx == -1 {
			log.Fatal(errors.New("ありえない値"))
		}
		// 使用中のURLを設定
		usingUrls[urlIdx] = true
		url := availableUrls[urlIdx]

		go func() {
			defer func() {
				outerBar.Increment()
				wg.Done()
				usingUrls[urlIdx] = false // ゴルーチンが終了したら使用中のURLを解放
				<-semaphore               // チャネルでURLが使用可能になったことを通知する
			}()
			jpSentences := translateSentences(outputDirPath, sentences, url)
			createSrt(outputDirPath, jpSentences)

			err := firebase.UploadJson(jpSentences, to.courceId, to.lectureId)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	wg.Wait()
	outerBar.Finish()
}

func getUnusedUrlIdx(availableUrls []string, usingUrls []bool) int {
	for i := range availableUrls {
		if isUsing := usingUrls[i]; !isUsing {
			return i
		}
	}
	return -1
}
