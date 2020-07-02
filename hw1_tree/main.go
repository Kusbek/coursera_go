package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

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

func dirTree(out io.Writer, path string, printFiles bool) error {
	tabs := ""
	err := printDirTree(out, path, printFiles, tabs)
	if err != nil {
		return err
	}
	return nil
}

func printDirTree(out io.Writer, path string, printFiles bool, tabs string) error {

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	if !printFiles {
		for i := 0; i < len(files); i++ {
			if !files[i].IsDir() {
				files = append(files[:i], files[i+1:]...)
				i--
			}
		}
	}

	for i, f := range files {
		fmt.Fprintf(out, "%v", tabs)
		last := false
		argtabs := tabs
		if i == len(files)-1 {
			last = true
			argtabs += "\t"
		} else {
			argtabs += "│\t"
		}
		myPrint(out, f, last)
		if f.IsDir() {
			err = printDirTree(out, path+"/"+f.Name(), printFiles, argtabs)
			if err != nil {
				return err
			}

		}
	}
	return nil
}

func myPrint(out io.Writer, f os.FileInfo, last bool) {
	custTab := "├───"
	if last {
		custTab = "└───"
	}
	if f.IsDir() {
		fmt.Fprintf(out, "%v%v\n", custTab, f.Name())
	} else {
		if f.Size() != 0 {
			fmt.Fprintf(out, "%v%v (%db)\n", custTab, f.Name(), f.Size())
		} else {
			fmt.Fprintf(out, "%v%v (empty)\n", custTab, f.Name())
		}

	}
}
