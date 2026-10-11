-- Drives Ableton Live's Export Audio/Video dialog through the UI.
-- Usage: osascript - <als-path> <output-dir> <file-name>
-- <output-dir> ends in a slash; <file-name> has no extension.
-- Requires the calling terminal to have Accessibility permission
-- (System Settings > Privacy & Security > Accessibility).

on run argv
	set alsPath to item 1 of argv
	set outDir to item 2 of argv
	set outName to item 3 of argv

	-- Same as double-clicking the file in Finder.
	-- Resolve the alias outside the tell block: inside it, Finder evaluates
	-- POSIX file itself and fails with -1728.
	set alsFile to (POSIX file alsPath) as alias
	tell application "Finder" to open alsFile

	-- Wait for Live to launch and open the Set: its window title is the
	-- file name without .als. Exact match, since slugs are prefix-nested.
	set setName to do shell script "basename " & quoted form of alsPath & " .als"
	set loaded to false
	repeat 120 times
		tell application "System Events"
			if exists process "Live" then
				tell process "Live"
					repeat with w in windows
						if name of w is setName then
							set loaded to true
							exit repeat
						end if
					end repeat
				end tell
			end if
		end tell
		if loaded then exit repeat
		delay 1
	end repeat
	if not loaded then error "timed out waiting for Live to open " & alsPath

	tell application "System Events"
		tell process "Live" to set frontmost to true
		delay 1

		-- option+2: Arrangement view
		key code 19 using {option down}
		delay 0.5

		-- cmd+a: select all in the arrangement
		keystroke "a" using {command down}
		delay 0.5

		-- cmd+shift+r: open the Export Audio/Video window
		keystroke "r" using {command down, shift down}
		delay 1.5

		-- return: start the export, which opens the save dialog
		key code 36
		delay 1.5

		-- cmd+shift+g: "Go to the folder" sheet. Typing a full path into
		-- the name field instead creates a file in / rather than navigating.
		keystroke "g" using {command down, shift down}
		delay 1
		keystroke outDir
		delay 0.5
		key code 36
		delay 1

		-- Name the file (replacing the default) and save.
		keystroke "a" using {command down}
		keystroke outName
		delay 0.5
		key code 36
	end tell
end run
