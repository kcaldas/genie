package ai

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kcaldas/genie/pkg/fileops"
	"github.com/kcaldas/genie/pkg/logging"
)

func CopyFile(data map[string]string, debug bool) (string, error) {
	source := data["source"]
	output := data[source]
	return output, nil
}

func ReadFile(data map[string]string, debug bool) (string, error) {
	filename := data["filename"]
	optional := data["optional"]
	
	fileManager := fileops.NewFileOpsManager()
	if optional == "true" && !fileManager.FileExists(filename) {
		logger := logging.NewComponentLogger("fileops")
		logger.Debug("optional file not found, returning empty string", "file", filename)
		return "", nil
	}
	
	fileData, err := fileManager.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(fileData), nil
}

func Expose(data map[string]string, debug bool) (string, error) {
	key := data["key"]
	value := data[key]
	return value, nil
}

func ReadText(data map[string]string, debug bool) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string

	ps1 := data["ps1"]

	fmt.Println(ps1)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()

		// If the user just hits ENTER (empty line), stop reading
		if line == "" {
			break
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}
