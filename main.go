package main

import (
	"fmt"
	"github.com/akamensky/argparse"
	"os"
	"io"
	"io/ioutil"
	"bufio"
	"regexp"
	"strings"
	"net/url"
	"net/http"
	"golang.org/x/net/html"
	"archive/zip"
	"path/filepath"
)

type AddOn struct {
	title string
	description string
	depends []string
}

func getDownloadLink(resp http.Response, content string) (string, error) {
	z := html.NewTokenizer(resp.Body)

	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			return "", fmt.Errorf("Download link not found!")
		case tt == html.StartTagToken:
			t := z.Token()

			next_tt := z.Next()
			if next_tt != html.TextToken {
				continue
			}
			text := string(z.Text())

			if t.Data == "a" {
				fmt.Printf("Checking link for %v == %v\n", text, content)
				if text != content {
					continue
				}
				for _, a := range t.Attr {
					if a.Key == "href" {
						return a.Val, nil
					}
				}
			}
		}
	}
}

func getCDNDownloadLink(resp http.Response) string {
	z := html.NewTokenizer(resp.Body)

	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			panic("Download link not found!")
		case tt == html.StartTagToken:
			t := z.Token()

			if t.Data == "iframe" {
				for _, a := range t.Attr {
					if a.Key == "src" {
						return a.Val
					}
				}
			}
		}
	}
}

func scanDirectory(path string) (*AddOn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return &addon, nil
}

func downloadPlugin(v, addon_dir, base_url, search_url string) error {
	// If there is a -, it's for a specific version
	parts := strings.Split(v, "-")
	v = parts[0]
	// Find the plugin from esoui.com
	values := url.Values{}
	values.Add("x", "0")
	values.Add("y", "0")
	values.Add("search", v)
	resp, err := http.PostForm(fmt.Sprintf("%s/%s", base_url, search_url), values)
	if err != nil {
		return err
	}
	download_link, err := getDownloadLink(*resp, "Download")
	resp.Body.Close()
	if err != nil {
		// Maybe we go the search page? In which case, follow the first link (most popular)
		fmt.Printf("Checking to see if we ended up in search page for %v...\n", v)
		resp, err := http.PostForm(fmt.Sprintf("%s/%s", base_url, search_url), values)
		download_link, err = getDownloadLink(*resp, v)
		resp.Body.Close()
		if err != nil {
			return err
		}
	}

	resp, err = http.Get(fmt.Sprintf("%s/%s", base_url, download_link))
	if err != nil {
		return err
	}
	download_link = getCDNDownloadLink(*resp)
	resp.Body.Close()


	resp, err = http.Get(download_link)
	if err != nil {
		return err
	}

	tmpfile, err := ioutil.TempFile("", "addon-zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	fmt.Printf("Downloading AddOn %s to %v\n", v, tmpfile.Name())

	io.Copy(tmpfile, resp.Body)

	fmt.Printf("Extracting file...\n")

	r, err := zip.OpenReader(tmpfile.Name())
	if err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(addon_dir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	addon_dir := "/media/data/SteamLibrary/steamapps/compatdata/306130/pfx/drive_c/users/steamuser/My Documents/Elder Scrolls Online/live/AddOns"
	base_url := "https://www.esoui.com"
	search_url := "/downloads/search.php"
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
			addon, err := scanDirectory(path)
			if err != nil {
				fmt.Printf("Got error in %v: %v\n", path, err)
				continue
			}
			addons[file.Name()] = *addon
			required = append(required, addon.depends...)
		}

		// Make sure all plugins are installed...
		for len(required) > 0 {
			v := required[0]
			if _, ok := addons[v]; !ok {
				fmt.Printf("Missing plugin: %v\n", v)

				err := downloadPlugin(v, addon_dir, base_url, search_url)
				if err != nil {
					fmt.Printf("Failed to install plugin %v: %v", v, err)
					required = required[1:]
					continue
				}

				fmt.Printf("Done!\n")
				path := fmt.Sprintf("%s/%s/%s.txt", addon_dir, v, v)
				addon, err := scanDirectory(path)
				if err != nil {
					fmt.Printf("Got error in %v: %v\n", path, err)
					continue
				}
				addons[v] = *addon
				required = append(required, addon.depends...)
			}
			required = required[1:]
		}
	}
}
