# fvtt-migrate

Moves a Foundry Virtual Tabletop installation from one machine to another and
afterwards compares the copy against the source.

It never writes to, deletes from or reads secrets out of the installation it
copies from. The licence key and the admin password hash are outside the area it
touches, and a test asserts it.

[Русская версия](README.ru.md)

## What you need

The program is needed on both machines. Download the file for your system from
the releases page.

- Windows, `fvtt-migrate-windows-amd64.exe`
- Linux, `fvtt-migrate-linux-amd64`
- Linux on ARM, `fvtt-migrate-linux-arm64`
- macOS, `fvtt-migrate-darwin-arm64`

On Linux and macOS mark it runnable once.

```
chmod +x fvtt-migrate-linux-amd64
```

Check that it answers.

```
fvtt-migrate version
```

You also need the path to your Foundry user data directory. It is the folder
containing `Data`, `Config` and `Logs`. The usual places are these.

- Windows, `%LOCALAPPDATA%\FoundryVTT`
- Linux, `~/.local/share/FoundryVTT`
- macOS, `~/Library/Application Support/FoundryVTT`

Return Foundry to the setup screen before you start. A world that is open is
being written to, and copying it while it is open damages it. The program checks
this and refuses rather than producing a broken copy.

## The simplest way

Run the downloaded file by double-clicking it. The program finds your Foundry
installation, starts the panel and opens it in a browser.

A small window appears with the address of the panel. It can be minimised, and
closing it stops the panel.

On Windows the first run raises a warning that the program has no publisher
signature. Choose more information, then run anyway.

If your Foundry installation is in an unusual place, the program says so and
shows where it looked. The path then has to be given once, with the command in
the panel section.

Everything else happens in the panel, and the rest of these instructions is for
people who prefer commands.

## Step 1. Look at what you have

```
fvtt-migrate scan --root /path/to/FoundryVTT
```

This reads and prints. It changes nothing and contacts nothing.

You get counts of worlds, systems and modules, how much of your data is actually
used, and two lists worth reading. The first, broken references. The second,
worlds whose game system is not installed. Such a world cannot open at the
destination, so the program leaves it behind.

## Step 2. Decide what moves

```
fvtt-migrate plan --root /path/to/FoundryVTT
```

This writes `plan.yaml`. Open it and read it. Nothing has moved yet.

The plan says which worlds travel, where every module comes from, and which
folders of loose files are included. You may edit it. If you edit it wrongly the
next step refuses to run and names the place that is wrong.

To also ask the package sources which versions exist now, add a flag.

```
fvtt-migrate plan --root /path/to/FoundryVTT --check-updates
```

Without that flag nothing leaves your machine. With it, only package manifests
are read.

If you are moving to a different Foundry generation, say so. The target version
changes every recommendation in the plan.

```
fvtt-migrate plan --root /path/to/FoundryVTT --target-core 14.363
```

## Step 3a. Move to another folder or disk

The simple case, and the one to try first.

```
fvtt-migrate apply --root /path/to/FoundryVTT --to /path/to/destination
```

To see what would happen without writing anything, add `--dry-run`.

## Step 3b. Move to a server over the internet

On the server, start the receiving side. It listens on the machine itself, and
your existing reverse proxy is what faces the internet. It prints an access key
of its own along with the command to run on the other machine.

```
fvtt-migrate serve --to /srv/foundry-data --listen 127.0.0.1:7788
```

The receiving side listens on the machine itself and is invisible from outside.
The only way in is the proxy, and it has to be configured.

The domain is usually taken by Foundry already, so it cannot be pointed at the
receiving side wholesale. The simplest arrangement gives the receiving side a
path of its own on the same domain. In Caddy it looks like this, and nothing
else needs changing.

```caddy
your.domain {
	handle_path /migrate/* {
		reverse_proxy 127.0.0.1:7788
	}
	handle {
		reverse_proxy 127.0.0.1:30000
	}
}
```

`handle_path` strips the prefix, so the receiving side sees ordinary addresses
and you give the destination with the prefix, `https://your.domain/migrate`.

The other arrangement is a name of its own, such as `migrate.your.domain`,
pointed at `127.0.0.1:7788` wholesale. The destination then carries no prefix.

In nginx three further settings have to change, or uploads fail immediately with
error 413.

```nginx
location /migrate/ {
    proxy_pass http://127.0.0.1:7788/;

    client_max_body_size 0;
    proxy_request_buffering off;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

Port 7788 does not need opening in the firewall.

On your own machine set the same key and give the `https` address.

```
export FVTT_MIGRATE_TOKEN=<the same key>
fvtt-migrate apply --root /path/to/FoundryVTT --to https://your.domain
```

Put the key in the variable rather than in the command. Command arguments are
visible to every other user of the machine.

If the connection drops, run the same command again. It continues rather than
starting over. The receiving side remembers what it already has, so a second run
sends only what is new.

## Step 4. Confirm it worked

This step is what makes the result trustworthy, so it is not skipped.

```
fvtt-migrate verify --root /path/to/FoundryVTT --to /path/to/destination
```

For a server, this way.

```
fvtt-migrate verify --root /path/to/FoundryVTT --to https://your.domain
```

Every world is opened on both sides and its documents are counted. Each line
should show the same number twice.

```
Worlds
  puti-azlanti      source=57078   target=57078   ok
```

A copy can place every file and still leave a world empty. Counting documents
finds exactly that case, which is why it is a separate step and why numbers are
printed rather than a tick.

To also re-read every byte at the destination, add `--deep`. That takes as long
as the copy did.

## When something goes wrong

`world "..." is loaded in Foundry`
Return Foundry to the setup screen and run again.

Error `413` from the server
The proxy limits the request body size. The nginx settings are above.

`sends the token in the clear`
Give the `https` address, not `http`.

`a token is required for a remote target`
Set `FVTT_MIGRATE_TOKEN` to the same value on both machines.

`the plan has errors`
The line and the reason are printed above the message. Fix `plan.yaml` and run
again.

`N source files changed during the copy`
Something wrote to the installation while it was being read. Close Foundry and
run again.

`verification failed`
The listed worlds did not arrive whole. Run `apply` again, it resends what is
missing.

## What it does not do

- It does not modify, move or delete anything in the source.
- It does not read `Config/license.json` or `Config/admin.txt`, and refuses to
  transfer them.
- It does not reach the network without `--check-updates` or an `https`
  destination.
- It does not update anything of its own accord. Every version change is one you
  chose in `plan.yaml`.

## Graphical panel

If you would rather not type commands, run the panel instead. Everything the
steps above do is available in it.

```
fvtt-migrate panel --root /path/to/FoundryVTT
```

Then open `http://127.0.0.1:7788/` in a browser. Closing the window that runs
the command stops the panel.

The panel is served only on this machine, because it has no password of its own.

It reads your installation and shows one page. At the top, the target Foundry
version, which is a control rather than a label, because changing it recomputes
every recommendation below. Then your worlds, with the ones that cannot open at
the destination switched off and the reason stated. Then all packages, sorted by
what should happen to each. Then the loose files, with any folder whose paths
have gone stale called out.

At the bottom you give the destination. A path for a folder on this machine, or
the `https` address of the receiving side together with the access key it
printed. Two buttons follow. The first works out what would move and writes
nothing. The second starts the transfer.

During the transfer the panel names the phase it is in and shows what has been
sent next to what was skipped because the far side already had it. Stopping is
safe at any moment.

When it finishes, the panel offers to check the result, which opens every world
on both sides and counts documents, the same check the `verify` command performs.

## Building from source

Needed only if you are changing the program. Requires Go 1.26 and Node 20.

```
./build-panel.sh
go build ./cmd/fvtt-migrate
```

## Licence

GNU GPL v3. The program works with your own campaign data, so you are entitled
to read what it does with it.
