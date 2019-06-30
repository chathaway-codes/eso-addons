# ESO Addon Manager

ESO runs great on Linux, but there is no reasonable plugin manager.

Huzzah! ESO Addon Manager was created!

## Installation

```
go get github.com/chathaway-codes/eso-addons
```

## Usage

```
$ eso-addons --help
ESO addon manager

Usage:
	eso-addons [options] list
	eso-addons [options] install (<plugin name>...)
	eso-addons [options] update

Options:
	-c, --config <path>	Config to load; defaults to ~/.eso_addons
		The config file is a TOML document which currently supports only
		one option; AddonsPath. Thhis should point to the ESO AddOns folder
	-p, --path <path>	Path to the ESO addons folder.

```

Plugin names will be searched for on https://www.esoui.com/; if there is not an exact match, installation will fail.
In that case, use an absolute URL to install the plugin.

## Setup

If you're running on Linux with a non-default Steam configuration, create a ~/.eso_addons with contents like:

```
AddonsPath = "~/.steam/steamapps/compatdata/306130/pfx/drive_c/users/steamuser/My Documents/Elder Scrolls Online/live/AddOns"
```

Updating the path as needed
