package firebase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud.google.com/go/storage"
	"github.com/sofuetakuma112/udmey-caps-translater/types"

	"google.golang.org/api/option"
)

var bucket *storage.BucketHandle

const bucketName string = "translate-udemy-42512.appspot.com"

func init() {
	ctx := context.Background()

	// 認証情報を設定
	opt := option.WithCredentialsFile("serviceAccountKey.json")

	// ストレージクライアントを作成
	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	bucket = client.Bucket(bucketName)
}

func UploadJson(sentences types.Sentences, courceId string, lectureId string) error {
	jsonData, err := json.Marshal(sentences)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// ストレージにJSONファイルをアップロード
	filePath := fmt.Sprintf("%v/%v/captions_ja_by_sentence.json", courceId, lectureId)
	obj := bucket.Object(filePath)
	w := obj.NewWriter(context.Background())
	w.ContentType = "application/json"
	_, err = w.Write(jsonData)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return nil
}
