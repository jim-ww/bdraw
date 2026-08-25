# bdraw

A mouse-first paint program for the terminal, built on [Bubble Tea](https://charm.land/bubbletea/).

## Features

- Full tool set: brush, rectangle, circle, line, arrow, eraser, fill, text, select/move, eyedropper
- Undo/redo, copy/paste, snap-to-grid, ctrl-drag to constrain lines/arrows to 45° or rect/circle to a square/perfect circle
- Tabs, recent-files list, crash recovery (autosave + `-restore` to resume the latest one)
- Save/load as JSON project files, export to PNG or SVG
- Optional SSH collaboration mode: `-collab` turns a running session into a multi-user room — everyone sees each other's cursor, tabs and edits sync live, and a `-collab-readonly` flag can restrict everyone but the host to viewing
- Configurable keybinds and a small color/behavior config file
- Usable without a mouse

## Install

Requires Go 1.26+.

```sh
go install github.com/jim-ww/bdraw@latest
```

Or try it with Nix:

```sh
nix run github:jim-ww/bdraw
```

Or add it to your flake inputs:

```nix
inputs.bdraw.url = "github:jim-ww/bdraw";
# environment.systemPackages = [ inputs.bdraw.packages.${system}.default ];
```

## Usage

```sh
bdraw                                  # blank canvas
bdraw file.json                        # open a project file
bdraw -c <configfile>                  # use a specific config file
bdraw -restore                         # resume the most recent autosave recovery file

bdraw -collab                          # start an SSH collaboration server (":2222" by default)
bdraw -collab -collab-addr host:port   # listen on a specific address
bdraw -collab -collab-readonly         # guests can view and follow along, but only the host can edit
```

To join a running collaboration session, just SSH into it — your login name becomes your display name next to your cursor:

```sh
ssh yourname@host -p 2222
```

Press `?` inside bdraw for the full keybind reference, or hover any toolbar button for its shortcut.

## Configuration

bdraw reads `config.json` from the default OS config directory unless `-c` is given:

```json
{
  "icons": false,
  "palette": ["#ffffff", "#ff0000", "#00ff00", "#0000ff"],
  "default_color": "#ffffff"
}
```

All fields are optional. Keybinds can be overridden individually in `keymap.json` in the same directory, keyed by action name:

```json
{
  "tool_brush": ["b"],
  "quit": ["ctrl+q", "q"]
}
```

`?` inside the app shows every action's current binding; the action names themselves are listed in `keymap.go`.

## Support the project

If bdraw is useful to you, consider a small donation.

**Monero (XMR)**

`83YGRqP8uHed6NeegZQeX9ccCxbzoRHHEEi7pTwk4aqdJZEVXXA6NWtetnsEM2v33zFBBt3Rp6DNhU9qhJEGPspU14yN8t7`

## License

GPL-3.0. Free to use, study, share, and modify — provided you keep the same freedoms for others.
