package main

import (
	"fmt"
	"path/filepath"
	"io/ioutil"
	"strings"
	"regexp"
	"crypto/md5"
	"os"
	"os/exec"
	"os/user"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OptArgs                []string
	OptCommand             string
	OptIdentifier          string   `long:"identifier" arg:"String" description:"indetify a file store the command result with given string"`
}

func runCmd(curFile *os.File, opts *options ) error {
	cmd := exec.Command(opts.OptCommand, opts.OptArgs...)
	cmd.Stdout = curFile
	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Wait()
	return nil
}

func runCopy(from string, to string ) error {
	cmd := exec.Command("cp", from, to)
	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Wait()
	return nil
}

func fileExists(filename string) bool {
    _, err := os.Stat(filename)
    return err == nil
}

func main() {
	os.Exit(_main())
}

func _main() (st int) {
	st = 1
	opts := &options{OptIdentifier: ""}
	parser := flags.NewParser(opts, flags.PassDoubleDash)
	parser.Name = "diff-detector"
	parser.Usage = "[OPTIONS] -- command args1 args2"

	args, err := parser.Parse()

	if err != nil || len(args) == 0 {
		parser.WriteHelp(os.Stdout)
		return
	}

	opts.OptCommand = args[0]
	if len(args) > 1 {
		opts.OptArgs = args[1:]
	}

	diffCmd, err := exec.LookPath("diff");
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	tmpDir := os.TempDir();

	hasher := md5.New()
	hasher.Write([]byte(opts.OptIdentifier))
	hasher.Write([]byte(opts.OptCommand))
	for _, v := range opts.OptArgs {
		hasher.Write([]byte(v))
	}
	commandKey := fmt.Sprintf("%x",hasher.Sum(nil))
	curUser, _ := user.Current()
	prevPath := filepath.Join(tmpDir, curUser.Uid + "-diff-detector-" + commandKey)
	// fmt.Printf("prevPath:%s diffCmd:%s\n",prevPath,diffCmd)

	curFile, err := ioutil.TempFile(tmpDir, "temp")
	if err != nil {
		fmt.Printf("%s", err)
		return
	}
	// fmt.Printf("curPath:%s\n",curFile.Name())
	defer os.Remove(curFile.Name())

	err = runCmd(curFile, opts)
	if err != nil {
		fmt.Printf("Error: %s",err)
		return
	}

	if ( ! fileExists(prevPath) ) {
		if ( len(opts.OptArgs) > 0 ) {
			fmt.Printf("Notice: first time execution command: '%s %s'\n", opts.OptCommand, strings.Join(opts.OptArgs, " "))
		} else {
			fmt.Printf("Notice: first time execution command: '%s'\n", opts.OptCommand)
		}
		err = runCopy(curFile.Name(), prevPath)
		if err != nil {
			fmt.Printf("Error: %s",err)
			return
		}
		st = 0
		return
	}

	// diff
	diffOut, diffError := exec.Command(diffCmd, "-U","1",prevPath,curFile.Name()).Output()
	err = runCopy(curFile.Name(), prevPath)
	if err != nil {
		fmt.Printf("Error: %s",err)
		return
	}
	// fmt.Printf("%s '%s'", diffOut, diffError);
	if ( diffError == nil ) {
		curOpen, err := os.Open(curFile.Name())
		if err != nil {
			fmt.Printf("Error: %s",err)
			return
		}
		defer curOpen.Close()
		fileinfo, _ := curOpen.Stat()
		data := make([]byte, 128)
		count, err := curOpen.Read(data)
		if err != nil {
			fmt.Printf("Error: %s",err)
			return
		}
		cur := string(data[0:count])
		cur = regexp.MustCompile("(\r\n|\r|\n)$").ReplaceAllString(cur, "")
		if ( fileinfo.Size() > 128 ) {
			fmt.Printf("OK: no difference: ```%s...```\n", cur)
		} else {
			fmt.Printf("OK: no difference: ```%s```\n", cur)
		}
		st = 0
	} else if ( regexp.MustCompile("exit status 1").MatchString(diffError.Error()) ) {
		diffRet := strings.Split(string(diffOut),"\n")
		diffRetString := strings.Join(diffRet[2:],"\n")
		diffRetString = regexp.MustCompile("(\r\n|\r|\n)$").ReplaceAllString(diffRetString, "")
		if ( len(diffRetString) > 512 ) {
			fmt.Printf("NG: detect difference: ```%s...```\n", diffRetString[0:512])
		} else {
			fmt.Printf("NG: detect difference: ```%s```\n", diffRetString)
		}
		st = 2
	} else {
		fmt.Printf("Error: %s\n", diffError)
	}

	return
}

