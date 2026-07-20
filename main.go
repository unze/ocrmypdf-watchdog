package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Context struct {
	InFolder       string
	BakFolder      string
	OutFolder      string
	OCRMyPDFBinary string
	Parameter      string
	Frequency      int
	Extensions     string
}

func main() {
	frequency, err := strconv.Atoi(os.Getenv("WATCHDOG_FREQUENCY"))
	if err != nil {
		frequency = 1
	}
	contextStruct := &Context{
		os.Getenv("OCRMYPDF_IN"),
		os.Getenv("OCRMYPDF_BAK"),
		os.Getenv("OCRMYPDF_OUT"),
		os.Getenv("OCRMYPDF_BINARY"),
		os.Getenv("OCRMYPDF_PARAMETER"),
		frequency,
		os.Getenv("WATCHDOG_EXTENSIONS"),
	}
	flag.StringVar(&contextStruct.InFolder, "in", contextStruct.InFolder, "input folder")
	flag.StringVar(&contextStruct.BakFolder, "bak", contextStruct.BakFolder, "backup folder")
	flag.StringVar(&contextStruct.OutFolder, "out", contextStruct.OutFolder, "output folder")
	flag.StringVar(&contextStruct.OCRMyPDFBinary, "ocrmypdf", contextStruct.OCRMyPDFBinary, "ocrmydpf binary to use")
	flag.IntVar(&contextStruct.Frequency, "frequency", frequency, "frequency in seconds")

	flag.Parse()

	if contextStruct.InFolder == "" || contextStruct.OutFolder == "" {
		log.Fatalln("in and/or out folder not defined.")
	}
	if contextStruct.OCRMyPDFBinary == "" {
		contextStruct.OCRMyPDFBinary = "ocrmypdf"
	}
	if contextStruct.Parameter == "" {
		contextStruct.Parameter = "-l eng+fra+deu --rotate-pages --deskew --jobs 4 --output-type pdfa"
	}
	if contextStruct.Extensions == "" {
		contextStruct.Extensions = "pdf,tif,tiff,jpg,jpeg,png,gif"
	}

	// FIX 1: Erzwinge ungepuffertes Logging direkt auf os.Stdout, um Hänger im Docker-Log-Buffer zu vermeiden
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stdout)

	log.Println("Watchdog configurations initialized.")
	log.Println("in = " + contextStruct.InFolder)
	log.Println("bak = " + contextStruct.BakFolder)
	log.Println("out = " + contextStruct.OutFolder)
	log.Printf("Frequency = %d seconds\n", contextStruct.Frequency)
	log.Println("Extensions to look for: " + contextStruct.Extensions)
	log.Println("OCRMyPDF binary = " + contextStruct.OCRMyPDFBinary)
	log.Println("OCRMyPDF parameter = " + contextStruct.Parameter)

	// FIX 2: Versionscheck mit hartem 5-Sekunden-Timeout absichern, falls das Binary beim Init blockiert
	log.Println("Checking OCRmyPDF version...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	versionCmd := exec.CommandContext(ctx, contextStruct.OCRMyPDFBinary, "--version")
	versionOut, versionErr := versionCmd.CombinedOutput()
	if versionErr != nil {
		log.Printf("Could not determine OCRmyPDF version (maybe timeout?): %v\n", versionErr)
	} else {
		log.Printf("OCRmyPDF Version: %s\n", strings.TrimSpace(string(versionOut)))
	}

	log.Println("Starting watchdog loop now...")
	contextStruct.watchdog()
}

func (c *Context) watchdog() {
	frequency := time.Duration(c.Frequency) * time.Second
	for {
		log.Println("Scanning folder " + c.InFolder + " for files...")
		var files []string
		err := filepath.Walk(c.InFolder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Error accessing path %s: %v\n", path, err)
				return err
			}
			if !info.IsDir() {
				if c.hasOneOfExtensions(path) {
					files = append(files, path)
				}
			}
			return nil
		})
		
		if err != nil {
			log.Printf("Walk failed: %v. Retrying in next cycle.\n", err)
		} else {
			log.Printf("Found %d matching files to process.\n", len(files))
			for _, file := range files {
				c.processDocument(file)
			}
		}

		log.Printf("Sleeping for %v...\n", frequency)
		timer := time.NewTimer(frequency)
		<-timer.C
		timer.Stop()
	}
}

func (c *Context) processDocument(path string) {
	log.Println("Processing file " + path)

	// Warte bis zu 10 Sekunden, falls die Datei noch vom Scanner/System geschrieben wird
	for i := 0; i < 10; i++ {
		if isFileReady(path) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// first get the parts of the path: dir+filename+ext
	directory := filepath.Dir(path)
	filename := filepath.Base(path)
	extension := filepath.Ext(filename)
	filename = filename[0 : len(filename)-len(extension)]
	
	// copy file to backup folder
	baktarget := c.BakFolder
	if !strings.HasSuffix(baktarget, "/") {
		baktarget = baktarget + "/"
	}
	targetWithExt := baktarget + filename + extension
	
	srcFile, err := os.Open(path)
	check(err)
	defer srcFile.Close()

	destFile, err := os.Create(targetWithExt) // creates if file doesn't exist
	check(err)
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile) // check first var for number of bytes copied
	check(err)

	err = destFile.Sync()
	check(err)

	// try to rename file
	tmpFile, err := ioutil.TempFile(directory, filename+".*."+extension)
	if err != nil {
		log.Printf("Unable to create temp file: %v", err)
		return
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())
	err = os.Rename(path, tmpFile.Name())
	if err != nil {
		log.Printf("Cannot rename file. Stopping here: %v", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	target := c.OutFolder
	if !strings.HasSuffix(target, "/") {
		target = target + "/"
	}
	targetWithoutExtension := target + filename
	target = targetWithoutExtension + ".tmp"
	log.Printf("Run command >%s %s %s %s<\n", c.OCRMyPDFBinary, c.Parameter, tmpFile.Name(), target)
	
	// FIX 3: Verhindert leere Argumente durch doppelte Leerzeichen in den Parametern
	rawArgs := strings.Split(c.Parameter, " ")
	var runargs []string
	for _, arg := range rawArgs {
		trimmed := strings.TrimSpace(arg)
		if trimmed != "" {
			runargs = append(runargs, trimmed)
		}
	}
	runargs = append(runargs, tmpFile.Name(), target)
	
	cmd := exec.Command(c.OCRMyPDFBinary, runargs...)

	// FIX 4: Streams direkt verbinden, um Fehlermeldungen von OCRmyPDF 17+ ungefiltert im Log zu sehen
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	log.Printf("Job finished with result %v\n", err)
	if err != nil {
		// error: tmp back to original name
		os.Rename(tmpFile.Name(), path)
	} else {
		// ok: rename tmp target to final target
		for fileExists(targetWithoutExtension+".pdf") {
			targetWithoutExtension += "_1"
		}
		os.Rename(target, targetWithoutExtension+".pdf")
	}
}

func (c *Context) hasOneOfExtensions(path string) bool {
	extensions := strings.Split(c.Extensions, ",")
	for _, s := range extensions {
		if strings.HasSuffix(strings.ToLower(path), "."+s) {
			return true
		}
	}
	return false
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func check(err error) {
	if err != nil {
		fmt.Printf("Error : %s\n", err.Error())
		os.Exit(1)
	}
}

// FIX 5: Nur lesend öffnen, damit es auch bei restriktiven Container-Usern (UID 1000) ohne Root klappt
func isFileReady(path string) bool {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	file.Close()
	return true
}
