// dotenv contains code from github.com/joho/godotenv
package dotenv

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const doubleQuoteSpecialChars = "\\\n\r\"!$`"

func Parse(r io.Reader) (map[string]string, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}
	return UnmarshalBytes(buf.Bytes())
}

func Load(filenames ...string) (err error) {
	filenames = filenamesOrDefault(filenames)
	for _, filename := range filenames {
		err = loadFile(filename, false)
		if err != nil {
			return
		}
	}
	return
}

func Overload(filenames ...string) (err error) {
	filenames = filenamesOrDefault(filenames)
	for _, filename := range filenames {
		err = loadFile(filename, true)
		if err != nil {
			return
		}
	}
	return
}

func Read(filenames ...string) (envMap map[string]string, err error) {
	filenames = filenamesOrDefault(filenames)
	envMap = make(map[string]string)
	for _, filename := range filenames {
		individualEnvMap, individualErr := readFile(filename)
		if individualErr != nil {
			err = individualErr
			return
		}
		for key, value := range individualEnvMap {
			envMap[key] = value
		}
	}
	return
}

func Unmarshal(str string) (envMap map[string]string, err error) {
	return UnmarshalBytes([]byte(str))
}

func UnmarshalBytes(src []byte) (map[string]string, error) {
	out := make(map[string]string)
	err := parseBytes(src, out)
	return out, err
}

func Exec(filenames []string, cmd string, cmdArgs []string, overload bool) error {
	op := Load
	if overload {
		op = Overload
	}
	if err := op(filenames...); err != nil {
		return err
	}
	command := exec.Command(cmd, cmdArgs...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func Write(envMap map[string]string, filename string) error {
	content, err := Marshal(envMap)
	if err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content + "\n")
	if err != nil {
		return err
	}
	return file.Sync()
}

func Marshal(envMap map[string]string) (string, error) {
	lines := make([]string, 0, len(envMap))
	for k, v := range envMap {
		if d, err := strconv.Atoi(v); err == nil {
			lines = append(lines, fmt.Sprintf(`%s=%d`, k, d))
		} else {
			lines = append(lines, fmt.Sprintf(`%s="%s"`, k, doubleQuoteEscape(v)))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func filenamesOrDefault(filenames []string) []string {
	if len(filenames) == 0 {
		return []string{".env"}
	}
	return filenames
}

func loadFile(filename string, overload bool) error {
	envMap, err := readFile(filename)
	if err != nil {
		return err
	}
	currentEnv := map[string]bool{}
	rawEnv := os.Environ()
	for _, rawEnvLine := range rawEnv {
		key := strings.Split(rawEnvLine, "=")[0]
		currentEnv[key] = true
	}
	for key, value := range envMap {
		if !currentEnv[key] || overload {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}

func readFile(filename string) (envMap map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	return Parse(file)
}

func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.ReplaceAll(line, string(c), toReplace)
	}
	return line
}
