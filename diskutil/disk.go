package diskutil

import (
	"encoding/json"
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
