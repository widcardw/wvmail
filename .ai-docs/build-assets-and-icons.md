# Build Assets & App Icons (macOS)

How the app icon is produced, why the Dock icon can silently stay stale, and how to
verify a change. Applies to Wails3 `v3.0.0-beta.25` on macOS 26.

## 1. Which file actually wins

`build/darwin/Info.plist` and `Info.dev.plist` contain **both** keys:

```xml
<key>CFBundleIconFile</key><string>icons</string>   <!-- Resources/icons.icns -->
<key>CFBundleIconName</key><string>appicon</string> <!-- Resources/Assets.car -->
```

macOS prefers the **asset catalog**: if `CFBundleIconName` is set *and* an
`Assets.car` containing that name is in `Contents/Resources`, `CFBundleIconFile`
and `icons.icns` are ignored entirely. So a fresh `icons.icns` changes nothing
while a stale `Assets.car` sits next to it.

`build/darwin/Taskfile.yml` (`run`) copies `Assets.car` **only if it exists**, so
deleting the file is a supported way to fall back to the `.icns`.

## 2. Icon generation

```bash
cd build
wails3 generate icons -input appicon.png \
  -macfilename darwin/icons.icns \
  -windowsfilename windows/icon.ico \
  -iconcomposerinput appicon.icon -macassetdir darwin
```

Two independent products:

| Output | Source | Needs |
|---|---|---|
| `darwin/icons.icns`, `windows/icon.ico` | `appicon.png` | nothing (pure Go) |
| `darwin/Assets.car` | `appicon.icon` (Icon Composer) | macOS 26 **and** `/usr/bin/actool` from a licensed Xcode 26+ |

## 3. The trap: `Assets.car` fails silently

`generateMacAsset` (`internal/commands/icons.go`) bails out with a
`macAssetNotSupportedError` when actool is missing, too old, or **when the Xcode
license has not been accepted**. `GenerateIcons` then swallows that error:

```go
if errors.As(err, &notSupported) {
    if options.Input == "" { return err } // only fatal without -input
    // otherwise: silent fallback to .icns/.ico generation
}
```

Net effect: with `-input appicon.png` present the command **exits 0**, refreshes
`icons.icns` + `icon.ico`, prints nothing, and leaves `Assets.car` untouched.

Two further reasons the dev path hides this:

- `darwin:run` (used by `wails3 dev`) has **no** `generate:icons` dependency — it
  just copies whatever is on disk, including a months-old `Assets.car`, and it
  never deletes files already inside the `.app`.
- `generate:icons`'s `generates:` list omits `darwin/Assets.car`, so once
  `icons.icns` is newer than the sources go-task reports the task as up to date
  and skips it. Force a rerun with `wails3 task common:generate:icons --force`.

### Diagnosing it

```bash
# is actool usable?
/usr/bin/actool --version          # "You have not agreed to the Xcode license agreements" ⇒ blocked
plutil -p /Library/Preferences/com.apple.dt.Xcode.plist | grep -i licence
plutil -p /Library/Preferences/com.apple.dt.Xcode.plist | grep -i AgreedToGMLicense
xcode-select -p                    # /Library/Developer/CommandLineTools ⇒ actool comes from Xcode.app
ls -la build/darwin/Assets.car     # stale mtime = the bug
xcrun assetutil --info build/darwin/Assets.car | head   # "Timestamp" tells when it was really built
```

The license plist records the Xcode version you agreed to
(`IDEXcodeVersionForAgreedToGMLicense`). After an Xcode upgrade it no longer
matches and **every** actool invocation fails until you re-accept.

### Second failure mode: Xcode upgrade left the shared resources behind

Accepting the license is not enough if the upgrade never ran Xcode's
*first launch* step. Xcode keeps shared developer frameworks in
`/Library/Developer/PrivateFrameworks`, installed by
`com.apple.pkg.XcodeSystemResources` — a package that lives *inside* Xcode.app
(`Contents/Resources/Packages/`) and is easy to leave un-installed. Symptom:

```
xcrun: error: unable to find utility "actool", not a developer tool or in PATH
xcrun: error: sh -c '.../xcodebuild -sdk ... -find actool' failed with exit code 65280
Error loading required libraries. If there is an ongoing installation please wait
for it to complete. Otherwise reinstall. (dlopen(@rpath/libxcodebuildLoader.dylib,
Symbol not found: _XPCTypeBool
  Referenced from: /Library/Developer/PrivateFrameworks/CoreDevice.framework/...
  Expected in:     /Library/Apple/System/Library/PrivateFrameworks/Mercury.framework/...)
```

`Mercury.framework` is the OS device-support framework; a stale `CoreDevice`
still imports `_XPCTypeBool`, which current macOS no longer exports. Every
`xcodebuild -find <tool>` then fails, so `xcrun` cannot resolve any developer
tool (this is a *toolchain-wide* breakage, not an actool-specific one).

```bash
# what the system has vs what this Xcode ships
pkgutil --pkg-info=com.apple.pkg.XcodeSystemResources | grep version   # e.g. 16.0.0.0.1.1726343358  ← stale
ls -la /Library/Developer/PrivateFrameworks/CoreDevice.framework/Versions/A/CoreDevice   # old mtime/size
# version of the package bundled inside Xcode:
pkgutil --expand /Applications/Xcode.app/Contents/Resources/Packages/XcodeSystemResources.pkg /tmp/xsr \
  && grep -o 'version="[^"]*"' /tmp/xsr/PackageInfo     # e.g. 27.0.0.0.1788430725

# repair (installs the packages bundled with the installed Xcode)
sudo xcodebuild -runFirstLaunch
# if xcodebuild's own tool lookup is already broken, install them directly:
sudo installer -pkg /Applications/Xcode.app/Contents/Resources/Packages/XcodeSystemResources.pkg -target /
sudo installer -pkg /Applications/Xcode.app/Contents/Resources/Packages/CoreTypes.pkg -target /
sudo installer -pkg /Applications/Xcode.app/Contents/Resources/Packages/MobileDevice.pkg -target /

xcrun actool --version | grep short-bundle-version   # works ⇒ done
```

The bundled `XcodeSystemResources.pkg` version is readable with
`pkgutil --expand <pkg> <dir>` and looking at `PackageInfo` (`version="..."`);
its payload is a `pbzx` stream (xz and raw chunks alternating around a cpio
archive) for anyone who wants to inspect what a package would install.

## 4. Regenerating `Assets.car`

```bash
cd build
wails3 generate icons -input appicon.png -macfilename darwin/icons.icns \
  -windowsfilename windows/icon.ico \
  -iconcomposerinput appicon.icon -macassetdir darwin
ls -la darwin/Assets.car                            # must be fresh now
```

Verified working again on 2026-09-27 (Xcode 27.0, `xcode-select -p` =
`/Applications/Xcode.app/Contents/Developer`): the run produced a 2.3 MB
`Assets.car` authored by `Xcode 27.0 (27A266a)` containing `appicon` `Icon Image`
renditions plus five `Color` renditions (the Icon Composer layers).

If `xcode-select -p` still points at `/Library/Developer/CommandLineTools`,
either switch it (`sudo xcode-select -s /Applications/Xcode.app/Contents/Developer`)
or prefix the command with `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`:
`/usr/bin/actool` is only a shim that resolves the tool through the *active*
developer directory, and the CLT install has no actool.

Note that when the Icon Composer path succeeds, the `.icns` is **actool's own**
export (≈56 KB) and the `-input`-based `.icns`/`.ico` generation is skipped for
the Mac side; a Go-generated `icons.icns` is ≈3.3 MB and pixel-identical at
normal sizes but carries no appearance variants.

Then relaunch the app (`wails3 dev` rebuilds the bundle, which copies the
catalog again) and, if the Dock still shows the old artwork, clear its cache:

```bash
killall Dock          # Dock re-reads icons; the app itself must be relaunched too
```

## 5. Fallback without Xcode (retired)

While actool was broken, `build/darwin/Assets.car` was moved aside
(`Assets.car.bak-20260504`) and the bundles fell back to `icons.icns`:

```bash
mv build/darwin/Assets.car build/darwin/Assets.car.bak-20260504
rm bin/*.app/Contents/Resources/Assets.car   # already-copied stale file inside the bundle
```

That is a valid emergency measure (the new art shows immediately, at the cost of
the layered/tintable appearance), but it is no longer needed: once §4 works, the
regenerated catalog wins again and the backup can be deleted. `CFBundleIconName`
can stay in the plists the whole time — it is ignored while no `Assets.car`
exists.

## 6. Verifying an icon change (do this instead of trusting the Dock)

```bash
# what macOS really resolves for a bundle
cat > /tmp/icon-dump.swift <<'EOF'
import AppKit
let img = NSWorkspace.shared.icon(forFile: CommandLine.arguments[1])
img.size = NSSize(width: 512, height: 512)
let rep = NSBitmapImageRep(data: img.tiffRepresentation!)!
try! rep.representation(using: .png, properties: [:])!
    .write(to: URL(fileURLWithPath: CommandLine.arguments[2]))
EOF
swift /tmp/icon-dump.swift bin/wvmail.dev.app /tmp/icon-dev.png

# or straight from the .icns
sips -s format png build/darwin/icons.icns --out /tmp/icon-icns.png
```

A/B the two states by copying the bundle to `/tmp` and dropping the stale
`Assets.car` back into `Contents/Resources` — the two renders differ if the
catalog is winning.

**Always use a fresh bundle path for each case.** `NSWorkspace.icon(forFile:)`
answers from the LaunchServices icon cache, so re-rendering the same `.app` path
after changing its `Resources/` returns the previous icon and makes the check
look like a no-op. The 2026-09-27 run used four throwaway copies
(`/tmp/wv1.app` … `wv4.app`) with these results:

| bundle contents | render |
|---|---|
| new `Assets.car` + new `icons.icns` | new art |
| new `Assets.car` only | new art |
| new `icons.icns` only | new art |
| **old** `Assets.car` + new `icons.icns` | new art *not* used — old "W" art |

The last row is the point: the catalog wins over the `.icns` whenever it is
present, which is exactly why a stale catalog silently masks a regenerated icon.

To render a dark-appearance variant (to confirm appearance renditions exist), set
`NSAppearance.current = NSAppearance(named: .darkAqua)` before reading the
image.

## 7. `build/` vs the installed CLI templates

`wails3 update build-assets` only refreshes the **templated** files listed under
the CLI's `internal/commands/updatable_build_assets/`:

```
darwin/Info.plist.tmpl  darwin/Info.dev.plist.tmpl  linux/**  windows/**
ios/**
```

It merges your existing plist values back in and **never touches Taskfiles, icon
art, or `Assets.car`**. It also sets `CFBundleIconName=appicon` automatically
whenever `build/darwin/Assets.car` exists.

For shared keys the freshly rendered template value wins; keys that exist only in
your current plist are kept. The regenerated plists are re-encoded by Go's plist
encoder, so their formatting (key order, indentation) changes even when the values
do not.

Known drift of this repo against the beta.25 templates:

- `build/darwin/` is missing `dmg-background.png`, `dmg-file-icon.icns`,
  `dmg-file-icon.png` — only needed by the newer `create:dmg` task.
- `build/Taskfile.yml` + `build/darwin/Taskfile.yml` come from an older template:
  no `wails3 tool docker-mounts`, no package-manager detection, no garble /
  obfuscated-build support, no DMG tasks, older `darwin:sign` (uses
  `SIGN_IDENTITY` vars instead of `wails3 setup`). Upgrading means copying the
  template files by hand; nothing in the icon pipeline depends on it.

## 8. Product identity / template placeholders

`build/config.yml` under `info:` is the single source of truth for the product
identity; everything else is generated from it. After editing `info:` (or
`fileAssociations:`), regenerate:

```bash
wails3 task common:update:build-assets
# = wails3 update build-assets -name wvmail -binaryname wvmail -config config.yml -dir .
```

That rewrites the *updatable* set from `config.yml`:

| file | notes |
|---|---|
| `darwin/Info.plist`, `darwin/Info.dev.plist` | `CFBundleName`, `CFBundleIdentifier`, `CFBundleGetInfoString`, `NSHumanReadableCopyright`, versions; the dev plist keeps `NSAppTransportSecurity` |
| `ios/Info.plist`, `ios/Info.dev.plist` | dev identifier gets a `.dev` suffix; macOS dev and prod share one identifier |
| `ios/LaunchScreen.storyboard`, `ios/project.pbxproj`, `ios/entitlements.plist` | launch-screen text, `PRODUCT_NAME`, `PRODUCT_BUNDLE_IDENTIFIER` |
| `linux/desktop`, `linux/nfpm/nfpm.yaml` | `Name`/`Comment`, package `name`/`vendor`/`description` |
| `windows/info.json`, `windows/nsis/wails_tools.nsh`, `windows/wails.exe.manifest` | exe version resource + NSIS defines |

Files the updater does **not** cover — edit these by hand, the edits stick:
`windows/nsis/project.nsi` (commented default overrides),
`windows/msix/app_manifest.xml`, `windows/msix/template.xml`, `ios/build.sh`,
`android/app/src/main/res/values/strings.xml` (`app_name`), and the signing
hints in `build/darwin/Taskfile.yml`.

Hand edits inside an *updatable* file are transient: they are overwritten on the
next run (e.g. `linux/nfpm/nfpm.yaml`'s `homepage: https://wails.io` comes
straight from the template and is not configurable via `config.yml`).

Current identity (set 2026-09-27): product `wvmail`, company `widcardw`,
identifier `win.widcard.wvmail` (reverse-DNS of the `widcard.win` domain),
version `0.0.1` — matching the hard-coded version string in
`frontend/src/views/settings/About.vue`. Changing the identifier does **not**
move user data: the app stores its config under `os.UserConfigDir()/wvmail`
(`~/Library/Application Support/wvmail`), which is independent of the bundle id.

Still carrying the upstream template default: the Android
`applicationId`/`namespace` `com.wails.app` in `build/android/app/build.gradle`
(with sources under `java/com/wails/app/`) — renaming it means moving that
directory too, so it was left alone.
