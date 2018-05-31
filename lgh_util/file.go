package lgh_util

import (
	"os"
	"path/filepath"
)

func FindConfigFile(fileName string) string {
	if _, err := os.Stat("./config/" + fileName); err == nil {

		fileName, _ = filepath.Abs("./config/" + fileName)

	} else if _, err := os.Stat("../config/" + fileName); err == nil {

		fileName, _ = filepath.Abs("../config/" + fileName)

	} else if _, err := os.Stat("../../config/" + fileName); err == nil {

		fileName, _ = filepath.Abs("../../config/" + fileName)

	}else if _, err := os.Stat("../../../config/" + fileName); err == nil {

		fileName, _ = filepath.Abs("../../../config/" + fileName)

	}else if _, err := os.Stat("../../../../config/" + fileName); err == nil {

		fileName, _ = filepath.Abs("../../../../config/" + fileName)

	} else if _, err := os.Stat("config/"+fileName); err == nil {

		fileName, _ = filepath.Abs("config/"+fileName)

	} else if _, err := os.Stat(fileName); err == nil {

		fileName, _ = filepath.Abs(fileName)

	}
	return fileName
}
