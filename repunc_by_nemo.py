from nemo.collections import nlp as nemo_nlp

import sys
import os

if __name__ == "__main__":
    args = sys.argv
    courceId = args[1]
    lectureId = args[2]
    captionTxtFileName = args[3]
    restoredPuncTxtFileName = args[4]

    currentDir = os.getcwd()
    targetDir = f"{currentDir}/captions/{courceId}/{lectureId}"
    f = open(f"{targetDir}/{captionTxtFileName}", "r")

    text = f.read()
    f.close()

    pretrained_model = nemo_nlp.models.PunctuationCapitalizationModel.from_pretrained("punctuation_en_bert")

    inference_results = pretrained_model.add_punctuation_capitalization(
        [
            text
        ],
        max_seq_length=128,
        step=8,
        margin=16,
        batch_size=32,
    )

    f = open(f"{targetDir}/{restoredPuncTxtFileName}", "w")
    f.write(inference_results[0])
    f.close()
