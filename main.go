package main

import (
	"fmt"
	"github.com/akamensky/argparse"
	"os"
	"io/ioutil"
	"bufio"
	"regexp"
	"strings"
)

type AddOn struct {
	title string
	description string
	depends []string
}

func main() {
	addon_dir := "/media/data/SteamLibrary/steamapps/compatdata/306130/pfx/drive_c/users/steamuser/My Documents/Elder Scrolls Online/live/AddOns"
	// Create new parser object
	parser := argparse.NewParser("eso-plugins", "Manages plugins for The Elder Scrolls online")
	// Create string flag
	list := parser.Flag("l", "list", &argparse.Options{Required: false, Help: "Print the known installed packages"})
	// Parse input
	err := parser.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		return
	}
	if *list {
		fmt.Printf("Walking dir %s\n", addon_dir)
		files, err := ioutil.ReadDir(addon_dir)
		if err != nil {
			panic(err)
		}
		addons := make(map[string]AddOn)
		required := []string{}
		for _, file := range files {
			fmt.Printf("Looking at %s\n", file.Name())
			path := fmt.Sprintf("%s/%s/%s.txt", addon_dir, file.Name(), file.Name())
			f, err := os.Open(path)
			if err != nil {
				fmt.Printf("Failed to open %s, moving on\n", path)
			}
			scanner := bufio.NewScanner(f)
			re := regexp.MustCompile(`## (.*): (.*)`)
			addon := AddOn{}
			for scanner.Scan() {
				line := scanner.Text()
				matches := re.FindStringSubmatch(line)
				if matches == nil {
					continue
				}
				switch matches[1] {
				case "Title":
					addon.title = matches[2]
				case "DependsOn":
					addon.depends = strings.Split(matches[2], " ")
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading standard input:", err)
			}
			fmt.Printf("Addon: %v\n", addon)
			addons[file.Name()] = addon
			required = append(required, addon.depends...)
		}

		// Make sure all plugins are installed...
		for _, v := range required {
			if _, ok := addons[v]; !ok {
				fmt.Printf("Missing plugin: %v\n", v)
			}
		}
	}
}
