package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/docopt/docopt-go"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var addon_dir string = ""

const base_url string = "https://www.esoui.com"
const search_url string = "/downloads/search.php"

type Config struct {
	AddonsPath string
}

type AddOn struct {
	title       string
	description string
	depends     []string
	version     string
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
		case "Version":
			addon.version = matches[2]
		case "Description":
			addon.description = matches[2]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &addon, nil
}

func downloadPlugin(v, addon_dir, base_url, search_url string) error {
	var resp *http.Response
	var err error
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		resp, err = http.Get(v)
	} else {
		// If there is a -, it's for a specific version
		parts := strings.Split(v, "-")
		v = parts[0]
		// Find the plugin from esoui.com
		values := url.Values{}
		values.Add("x", "0")
		values.Add("y", "0")
		values.Add("search", v)
		resp, err = http.PostForm(fmt.Sprintf("%s/%s", base_url, search_url), values)
		if err != nil {
			return err
		}
	}
	download_link, err := getDownloadLink(*resp, "Download")
	resp.Body.Close()
	if err != nil {
		fmt.Printf("Failed to find plugin %v; consider just pasting a link\n", v)
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

func updatePlugins(force_install bool) {
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
		if _, ok := addons[v]; !ok || force_install {
			fmt.Printf("Updating plugin: %v\n", v)

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

func main() {

	usage := `ESO addon manager

Usage:
	eso-addons [options] list
	eso-addons [options] install (<plugin name>...)
	eso-addons [options] update

Options:
	-c, --config <path>	Config to load; defaults to ~/.eso_addons
		The config file is a TOML document which currently supports only
		one option; AddonsPath. Thhis should point to the ESO AddOns folder
	-p, --path <path>	Path to the ESO addons folder.
	`
	// Create new parser object
	arguments, err := docopt.ParseDoc(usage)
	if err != nil {
		panic(err)
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	if arguments["--config"] == nil {
		arguments["--config"] = path.Join(user.HomeDir, ".eso_addons")
	}
	if arguments["--path"] == nil {
		var conf Config
		if runtime.GOOS == "windows" {
			arguments["--path"] = path.Join(user.HomeDir, "My Documents", "Elder Scrolls Online", "live", "AddOns")
		} else {
			arguments["--path"] = path.Join(user.HomeDir, ".steam", "steamapps", "compatdata", "306130", "pfx", "drive_c", "users", "steamuser", "My Documents", "Elder Scrolls Online", "live", "AddOns")
		}
		if _, err := toml.DecodeFile(arguments["--config"].(string), &conf); err != nil {
			fmt.Printf("Warning; failed to find config file %v\n", arguments["--config"].(string))
			// No idea; just select the default path
		} else if conf.AddonsPath != "" {
			arguments["--path"] = conf.AddonsPath
		}
	}

	addon_dir = arguments["--path"].(string)
	if arguments["install"].(bool) {
		plugin_names := arguments["<plugin name>"].([]string)
		for _, plugin_name := range plugin_names {
			err := downloadPlugin(plugin_name, addon_dir, base_url, search_url)
			if err != nil {
				fmt.Printf("Failed to install plugin %v: %v", plugin_name, err)
				return
			}
			updatePlugins(false)
		}
		fmt.Printf("Done!\n")

	} else if arguments["list"].(bool) {
		fmt.Printf("Walking dir %s\n", addon_dir)
		files, err := ioutil.ReadDir(addon_dir)
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			path := fmt.Sprintf("%s/%s/%s.txt", addon_dir, file.Name(), file.Name())
			addon, err := scanDirectory(path)
			if err != nil {
				fmt.Printf("Got error in %v: %v\n", path, err)
				continue
			}
			fmt.Printf("%v %v -- %v\n", addon.title, addon.version, addon.description)
		}
	} else if arguments["update"].(bool) {
		updatePlugins(true)
	}
}
