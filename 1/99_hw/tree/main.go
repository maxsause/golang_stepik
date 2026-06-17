package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	// "strings"
)

func dirTree(out io.Writer, path string, printFiles bool) error {
	return recursTree(out, path, printFiles, "")
}

func recursTree(out io.Writer, path string, printFiles bool, prefix string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	entries := []os.DirEntry{}
	for _, file := range files {
		if file.IsDir() || printFiles {
			entries = append(entries, file)
		}
	}

	for i, file := range entries {
		newPath := filepath.Join(path, file.Name())
		isLast := i == len(entries)-1

		connector := "├───"
		newPrefix := prefix + "│\t"
		if isLast {
			connector = "└───"
			newPrefix = prefix + "\t"
		}

		if file.IsDir() {
			fmt.Fprintf(out, "%s%s%s\n", prefix, connector, file.Name())
			err := recursTree(out, newPath, printFiles, newPrefix)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
		} else if printFiles {
			info, err := os.Stat(newPath)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			size := ""
			if info.Size() == 0 {
				size = "empty"
			} else {
				size = fmt.Sprintf("%db", info.Size())
			}
			fmt.Fprintf(out, "%s%s%s (%s)\n", prefix, connector, file.Name(), size)
		}
	}
	return nil
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
