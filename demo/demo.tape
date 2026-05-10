# pkty demo — recorded with VHS (https://github.com/charmbracelet/vhs)
#
# Prerequisites:
#   1. Build pkty:          go build -o pkty ./cmd/pkty
#   2. Generate demo pcap:  go run demo/gen_pcap.go
#   3. Install VHS:         brew install vhs  (or https://github.com/charmbracelet/vhs)
#   4. Record:              vhs demo/demo.tape

Output demo/demo.gif

Set Shell "bash"
Set FontSize 13
Set FontFamily "JetBrains Mono"
Set Width 1800
Set Height 900
Set Theme "Catppuccin Mocha"
Set Padding 16
Set PlaybackSpeed 1.0
Set TypingSpeed 80ms

# --- start pkty in pcap replay mode (no sudo needed) ---
Type "./pkty -r demo/demo.pcap"
Sleep 200ms
Enter
Sleep 3s

# --- packet list is populating, owl is animating ---
# scroll down in the packet list a few times
Sleep 500ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 400ms

# --- focus DNS widget (key 3) ---
Type "3"
Sleep 600ms

# scroll down to a DNS entry
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 400ms

# set a display filter from DNS row
Enter
Sleep 1200ms

# --- filter is active — packet list narrows ---
Sleep 800ms

# clear the filter
Type "D"
Sleep 800ms

# --- focus bottom panel (key 4) ---
Type "4"
Sleep 600ms

# cycle to Remote Hosts
Tab
Sleep 700ms

# cycle to TLS Inspector
Tab
Sleep 700ms

# --- toggle to NetGraph (n key, from any focus) ---
Type "1"
Sleep 300ms
Type "n"
Sleep 1500ms

# scroll NetGraph
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "k"
Sleep 500ms

# --- back to packet inspector ---
Type "n"
Sleep 600ms
Type "1"
Sleep 600ms

# --- quit ---
Type "q"
Sleep 500ms
