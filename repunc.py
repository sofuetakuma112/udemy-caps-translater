import sys
import os
from rpunct import RestorePuncts

if __name__ == "__main__":
    print("句読点モデルに渡して句読点を予測する")
    args = sys.argv
    courceId = args[1]
    lectureId = args[2]

    currentDir = os.getcwd()
    targetDir = f"{currentDir}/captions/{courceId}/{lectureId}"
    f = open(f"{targetDir}/textPuncEscaped.txt", "r")

    textPuncEscaped = f.read()
    f.close()

    rpunct = RestorePuncts()
    textPuncEscapedAndRestored = rpunct.punctuate(textPuncEscaped)

    f = open(f"{targetDir}/textPuncEscapedAndRestored.txt", "w")
    f.write(textPuncEscapedAndRestored)
    f.close()
