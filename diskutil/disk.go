package diskutil

import (
	"encoding/json"
	"log"
	"os"
)

func WriteStructToDisk(data []byte, filename string) error {

	err := os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func ReadStructFromDisk(filename string, data interface{}) error {
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonData, data)
	if err != nil {
		return err
	}
	return nil
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func SavePostToDisk(fmtdstring string) {
	f, err := os.OpenFile("posts.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmtdstring + "\n\n"); err != nil {
		log.Println(err)
	}
}
