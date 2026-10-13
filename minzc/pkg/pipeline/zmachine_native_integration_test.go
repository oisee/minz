package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Real Nanz interpreter executes an independent Inform-built Z3 story from
// physical bank0. Host I/O is replaced by explicit native capture boundaries.
func TestZMachineBankedInterpreterNative(t *testing.T) {
	testZMachineBankedInterpreterNative(t, false, "", "")
}
func TestZMachineBankedHighMemoryNative(t *testing.T) {
	testZMachineBankedInterpreterNative(t, true, "", "")
}
func TestZMachineBankedZorkStartupNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to an independently supplied Zork I story")
	}
	testZMachineBankedInterpreterNative(t, false, path, "")
}
func TestZMachineBankedZorkGameplayNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to independently supplied Zork I story")
	}
	testZMachineBankedInterpreterNative(t, false, path, "look\nopen mailbox\ntake leaflet\nread leaflet\ninventory\nquit\ny\n")
}
func TestZMachineBankedZorkConsoleNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-startup")
}
func TestZMachineBankedZorkKeyboardNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-keyboard")
}
func TestZMachineBankedZorkExploreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY")
	}
	if os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_VM for original-interpreter transcript reference")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-explore")
}
func TestZMachineBankedZorkRestartNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-restart")
}
func TestZMachineBankedZorkSaveRestoreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-save")
}
func TestZMachineBankedZork128WindowsSaveRestoreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-save")
}
func TestZMachineBankedZorkCellarSaveRestoreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-cellar-save")
}

func TestZMachineBankedZork128WindowsCellarLifecycleNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-cellar")
}

// A treasure changes object ownership and score, independently checked with
// Frotz as well as the original VM path. This is not a complete walkthrough.
func TestZMachineBankedZork128TreasureNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" || os.Getenv("MINZ_Z3_FROTZ") == "" {
		t.Skip("set story, VM and Frotz")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-treasure")
}

func TestZMachineBankedZork128TreasureRestoreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-treasure-save")
}

func TestZMachineBankedZork128EarlyDungeonNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-dungeon")
}

func TestZMachineBankedZork128DamLobbyNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-dam")
}

func TestZMachineBankedZork128CrystalSkullNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-skull")
}

func TestZMachineBankedZork128CyclopsNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-cyclops")
}

func TestZMachineBankedZork128TridentNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-trident")
}

func TestZMachineBankedZork128TreasureRoomNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-treasure-room")
}

func TestZMachineBankedZork128ColdPassageNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-cold-passage")
}

func TestZMachineBankedZork128FullWalkthroughNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-full-walkthrough")
}

func TestZMachineBankedZork128LateSaveWalkthroughNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-windows-late-save")
}

func TestZMachineBankedZorkII128BankNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-bank")
}

func TestZMachineBankedZorkII128PrincessTreasureNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-princess-treasure")
}

func TestZMachineBankedZorkII128PrincessNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-princess")
}

func TestZMachineBankedZorkII128DragonNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-dragon")
}

func TestZMachineBankedZorkII128KeyNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-key")
}

func TestZMachineBankedZorkII128ReturnWellNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-return-well")
}

func TestZMachineBankedZorkII128RobotNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-robot")
}

func TestZMachineBankedZorkII128CakesNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-cakes")
}

func TestZMachineBankedZorkII128WellNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-well")
}

func TestZMachineBankedZorkII128RiddleNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-riddle")
}

func TestZMachineBankedZorkII128GardenNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-garden")
}

// A separately built full V3 story, not the Zork I workload used earlier.
func TestZMachineBankedZorkIIGameplayNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" || os.Getenv("MINZ_Z3_FROTZ") == "" {
		t.Skip("set MINZ_Z3_ZORK2_STORY, MINZ_Z3_VM and MINZ_Z3_FROTZ")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2")
}
func TestZMachineBankedZorkIICompressedNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" || os.Getenv("MINZ_Z3_FROTZ") == "" {
		t.Skip("set MINZ_Z3_ZORK2_STORY, MINZ_Z3_VM and MINZ_Z3_FROTZ")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-compressed")
}
func TestZMachineBankedZorkIICompressedRestartNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" || os.Getenv("MINZ_Z3_FROTZ") == "" {
		t.Skip("set Zork II story, VM and Frotz")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-compressed-restart")
}
func TestZMachineBankedZorkIICompressedSaveNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-compressed-save")
}
func TestZMachineBankedZorkIICompressedSaveRestartNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-compressed-save-restart")
}
func TestZMachineBankedZorkII128WindowsLifecycleNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set Zork II story and VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-128-windows")
}

func testZMachineBankedInterpreterNative(t *testing.T, highMemory bool, externalStory string, commands string) {
	root := filepath.Clean("../../..")
	story, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/stream3.z3"))
	if err != nil {
		t.Fatal(err)
	}
	if highMemory {
		fixture := filepath.Join(t.TempDir(), "high.z3")
		cmd := exec.Command("python3", filepath.Join(root, "examples/zmachine/z3-fixtures/high_memory.py"), fixture, "--bytes", "98302")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v %s", err, out)
		}
		story, err = os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
	}
	if externalStory != "" {
		story, err = os.ReadFile(externalStory)
		if err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "core.nanz")
	cmd := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), path, "--story-bytes", strconv.Itoa(len(story)))
	zorkIIBank := commands == "@console-zork2-bank"
	zorkIIPrincessTreasure := commands == "@console-zork2-princess-treasure" || zorkIIBank
	zorkIIPrincess := commands == "@console-zork2-princess" || zorkIIPrincessTreasure
	zorkIIDragon := commands == "@console-zork2-dragon" || zorkIIPrincess
	zorkIIKey := commands == "@console-zork2-key" || zorkIIDragon
	zorkIIReturnWell := commands == "@console-zork2-return-well" || zorkIIKey
	zorkIIRobot := commands == "@console-zork2-robot" || zorkIIReturnWell
	zorkIICakes := commands == "@console-zork2-cakes" || zorkIIRobot
	zorkIIWell := commands == "@console-zork2-well" || zorkIICakes
	zorkIIRiddle := commands == "@console-zork2-riddle" || zorkIIWell
	zorkIIGarden := commands == "@console-zork2-garden" || zorkIIRiddle
	if zorkIIGarden {
		commands = "@console-zork2-compressed"
	}
	lateSave := commands == "@console-windows-late-save"
	fullWalkthrough := commands == "@console-windows-full-walkthrough" || lateSave
	coldPassage := commands == "@console-windows-cold-passage"
	treasureRoom := commands == "@console-windows-treasure-room"
	trident := commands == "@console-windows-trident"
	cyclops := commands == "@console-windows-cyclops"
	crystalSkull := commands == "@console-windows-skull"
	damLobby := commands == "@console-windows-dam"
	earlyDungeon := commands == "@console-windows-dungeon" || damLobby || crystalSkull || cyclops || trident || treasureRoom || coldPassage || fullWalkthrough
	zorkIIWindows := commands == "@console-zork2-128-windows"
	if zorkIIWindows {
		commands = "@console-zork2-compressed-save-restart"
	}
	treasureSave := commands == "@console-windows-treasure-save"
	treasure := commands == "@console-windows-treasure" || treasureSave
	windowCellar := commands == "@console-windows-cellar"
	windows := commands == "@console-windows-save" || windowCellar || treasure || zorkIIWindows || earlyDungeon || zorkIIGarden
	if windows && !zorkIIWindows && !zorkIIGarden {
		commands = "@console-save"
		if (treasure && !treasureSave) || earlyDungeon {
			commands = "@console-explore"
		}
		if lateSave {
			commands = "@console-save"
		}
		if windowCellar {
			commands = "@console-cellar-save"
		}
	}
	combinedLifecycle := commands == "@console-zork2-compressed-save-restart"
	compressedSave := commands == "@console-zork2-compressed-save" || combinedLifecycle
	compressedRestart := commands == "@console-zork2-compressed-restart" || combinedLifecycle
	compressedStory := commands == "@console-zork2-compressed" || compressedRestart || compressedSave
	zorkII := commands == "@console-zork2" || compressedStory
	cellarSave := commands == "@console-cellar-save"
	saveRestore := commands == "@console-save" || cellarSave || compressedSave
	restart := commands == "@console-restart"
	explore := commands == "@console-explore" || restart || saveRestore || zorkII
	keyboard := commands == "@console-keyboard" || explore
	console := commands == "@console-startup" || keyboard
	var restartArchive []byte
	if (restart || saveRestore) && !compressedSave {
		archivePath := filepath.Join(t.TempDir(), "restart.rle")
		pack := exec.Command("python3", filepath.Join(root, "scripts/zmachine/restart_rle.py"), externalStory, archivePath)
		if out, err := pack.CombinedOutput(); err != nil {
			t.Fatalf("pack %v %s", err, out)
		}
		var err error
		restartArchive, err = os.ReadFile(archivePath)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "--restart-bytes", strconv.Itoa(int(story[14])<<8|int(story[15])), "--restart-rle-bytes", strconv.Itoa(len(restartArchive)))
	}

	if saveRestore && !compressedSave {
		cmd.Args = append(cmd.Args, "--save-story", externalStory)
	}
	if console {
		commands = ""
		cmd.Args = append(cmd.Args, "--console")
		if windows {
			cmd.Args = append(cmd.Args, "--windows")
		}
	}
	if commands != "" {
		commandPath := filepath.Join(t.TempDir(), "commands.txt")
		if err := os.WriteFile(commandPath, []byte(commands), 0600); err != nil {
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "--commands", commandPath)
	}
	var storageImage []byte
	if compressedStory {
		layoutDir := t.TempDir()
		layout := exec.Command("python3", filepath.Join(root, "scripts/zmachine/story_lz_layout.py"), externalStory, layoutDir)
		if compressedRestart || compressedSave {
			layout.Args = append(layout.Args, "--archive-blocks")
		}
		if out, err := layout.CombinedOutput(); err != nil {
			t.Fatalf("layout: %v %s", err, out)
		}
		storageImage, err = os.ReadFile(filepath.Join(layoutDir, "layout-2048.bin"))
		if err != nil {
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "--story-layout", filepath.Join(layoutDir, "layout-2048.json"))
		if compressedSave {
			cmd.Args = append(cmd.Args, "--layout-save-story", externalStory)
		}
		if compressedRestart {
			cmd.Args = append(cmd.Args, "--layout-restart")
		}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib")})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MINZ_Z3_ASM") != "" {
		if err := os.WriteFile(os.Getenv("MINZ_Z3_ASM"), []byte(steps.Assembly), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	if 0x5b00+len(a.Binary) > 0xbe00 {
		t.Fatalf("resident core reaches %04x; native stack starts be00", 0x5b00+len(a.Binary))
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if console {
		for ch := 32; ch < 128; ch++ {
			for line := 0; line < 8; line++ {
				m.Memory.ROM[1][0x3d00+(ch-32)*8+line] = byte((ch*17 + line*29) ^ 0x5a)
			}
		}
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	for i, b := range story {
		m.Memory.RAM[pages[i/16384]][i%16384] = b
	}
	if compressedStory {
		for i, b := range storageImage {
			m.Memory.RAM[pages[i/16384]][i%16384] = b
		}
	}
	if (restart || saveRestore) && !compressedSave {
		for i, b := range restartArchive {
			at := len(story) + i
			m.Memory.RAM[pages[at/16384]][at%16384] = b
		}
	}
	m.Memory.SetPagingForce(0x10)
	m.Memory.Write(0xbf10, 0x76, false)
	m.Memory.Write(0xbf00, 0x10, false)
	m.Memory.Write(0xbf01, 0xbf, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	m.CPU.SetTstates(0)
	if keyboard {
		snapshot := filepath.Join(t.TempDir(), "keyboard.sna")
		if err := formats.SaveSNA(snapshot, m); err != nil {
			t.Fatal(err)
		}
		saved, err := formats.LoadSNA(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := spectrum.NewPentagon128(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		fresh.Memory.ROM = m.Memory.ROM
		formats.ApplySnapshot(fresh, saved)
		if fresh.Memory.RAM != m.Memory.RAM || fresh.CPU.PC() != m.CPU.PC() || fresh.CPU.SP() != m.CPU.SP() {
			t.Fatal("snapshot before keyboard differs")
		}
		m = fresh
	}
	if zorkIIWindows {
		verify := func(want byte) int {
			m.CPU.SetPC(uint16(a.Symbols["verify_story"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetTstates(0)
			for i := 0; i < 100000000; i++ {
				if m.CPU.PC() == 0xbf10 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			if m.CPU.PC() != 0xbf10 || m.CPU.SP() != 0xbf02 || byte(m.CPU.AF()>>8) != want || m.Memory.PagingState() != 0x10 {
				t.Fatalf("checksum verification result PC/SP %04x/%04x A %d want %d", m.CPU.PC(), m.CPU.SP(), byte(m.CPU.AF()>>8), want)
			}
			return m.CPU.Tstates()
		}
		// verify_story validates against the runtime story_size populated by main.
		sizeAddr := uint16(a.Symbols["story_size"])
		for i := 0; i < 4; i++ {
			m.Memory.Write(sizeAddr+uint16(i), byte(len(story)>>(8*i)), false)
		}
		ticks := verify(1)
		m.Memory.RAM[0][64] ^= 1
		verify(0)
		m.Memory.RAM[0][64] ^= 1
		t.Logf("full compressed checksum %d T; one-byte corruption rejected", ticks)
		m.Memory.Write(uint16(a.Symbols["native_story_cache_key"]), 255, false)
		m.Memory.Write(uint16(a.Symbols["native_story_cache_misses"]), 0, false)
		m.Memory.Write(uint16(a.Symbols["native_story_cache_misses"])+1, 0, false)
		m.CPU.SetPC(uint16(a.Symbols["main"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetTstates(0)
	}
	keyText := []byte("look\ropen mailbox\rtake leaflet\rread leaflet\rinventory\rquit\ry\r")
	var reference []byte
	if explore {
		route := []byte("north\neast\nopen window\nenter window\nwest\ntake lamp\ntake sword\ninventory\nturn on lamp\nmove rug\nopen trap door\ndown\nlook\nup\nquit\ny\n")
		if treasure {
			route = []byte("north\nnorth\nclimb tree\ntake egg\ndown\nsouth\neast\nopen window\nenter window\nwest\nopen trophy case\nput egg in case\nlook in case\nscore\ninventory\nquit\ny\n")
		}
		if zorkII {
			route = []byte("look\ninventory\ntake lamp\nturn on lamp\nnorth\nlook\nsouth\nquit\ny\n")
		}
		if compressedRestart {
			route = []byte("take lamp\nturn on lamp\nsouth\nrestart\ny\nlook\ninventory\nquit\ny\n")
		}
		if restart {
			route = []byte("open mailbox\ntake leaflet\nrestart\ny\nlook\ninventory\nquit\ny\n")
		}
		if saveRestore {
			route = []byte("open mailbox\nsave\ntake leaflet\ninventory\nrestore\ninventory\nquit\ny\n")
		}
		if compressedSave {
			route = []byte("take lamp\ntake sword\nturn on lamp\nsouth\nsave\ndrop sword\ninventory\nrestore\ninventory\nquit\ny\n")
		}
		if combinedLifecycle {
			route = []byte("take lamp\ntake sword\nturn on lamp\nsouth\nsave\ndrop sword\ninventory\nrestore\ninventory\nrestart\ny\nlook\ninventory\nquit\ny\n")
		}
		if cellarSave {
			route = []byte("north\neast\nopen window\nenter window\nwest\ntake lamp\ntake sword\nturn on lamp\nmove rug\nopen trap door\ndown\nsave\ndrop sword\ninventory\nrestore\ninventory\ndrop lamp\nrestore\ninventory\nquit\ny\n")
		}
		if treasureSave {
			route = []byte("north\nnorth\nclimb tree\ntake egg\ndown\nsouth\neast\nopen window\nenter window\nwest\nopen trophy case\nput egg in case\nsave\ntake egg from case\nlook in case\ninventory\nrestore\nlook in case\nscore\ninventory\nquit\ny\n")
		}
		if windowCellar {
			route = bytes.Replace(route, []byte("quit\ny\n"), []byte("restart\ny\nlook\ninventory\nquit\ny\n"), 1)
		}
		if earlyDungeon {
			routeFile := "early-dungeon.txt"
			if damLobby {
				routeFile = "dam-lobby.txt"
			}
			if crystalSkull {
				routeFile = "crystal-skull.txt"
			}
			if cyclops {
				routeFile = "cyclops.txt"
			}
			if trident {
				routeFile = "trident.txt"
			}
			if treasureRoom {
				routeFile = "treasure-room.txt"
			}
			if coldPassage {
				routeFile = "cold-passage.txt"
			}
			if fullWalkthrough {
				routeFile = "full-walkthrough.txt"
			}
			if lateSave {
				routeFile = "late-save-walkthrough.txt"
			}
			route, err = os.ReadFile(filepath.Join(root, "examples/zmachine/zork-west-demo/routes", routeFile))
			if err != nil {
				t.Fatal(err)
			}
		}
		if zorkIIGarden {
			routeFile := "zork2-garden.txt"
			if zorkIIRiddle {
				routeFile = "zork2-riddle.txt"
			}
			if zorkIIWell {
				routeFile = "zork2-well.txt"
			}
			if zorkIICakes {
				routeFile = "zork2-cakes.txt"
			}
			if zorkIIRobot {
				routeFile = "zork2-robot.txt"
			}
			if zorkIIReturnWell {
				routeFile = "zork2-return-well.txt"
			}
			if zorkIIKey {
				routeFile = "zork2-key.txt"
			}
			if zorkIIDragon {
				routeFile = "zork2-dragon.txt"
			}
			if zorkIIPrincess {
				routeFile = "zork2-princess.txt"
			}
			if zorkIIPrincessTreasure {
				routeFile = "zork2-princess-treasure.txt"
			}
			if zorkIIBank {
				routeFile = "zork2-bank.txt"
			}
			route, err = os.ReadFile(filepath.Join(root, "examples/zmachine/zork-west-demo/routes", routeFile))
			if err != nil {
				t.Fatal(err)
			}
		}
		d := t.TempDir()
		source, err := os.ReadFile(filepath.Join(root, "examples/zmachine/zork-west-demo/zvm.nanz"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "zvm.nanz"), source, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "zork-west-demo.z3"), story, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(d, "text"), 0700); err != nil {
			t.Fatal(err)
		}
		printSource, err := os.ReadFile(filepath.Join(root, "stdlib/text/print.nanz"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "text/print.nanz"), printSource, 0600); err != nil {
			t.Fatal(err)
		}
		vm := exec.Command(os.Getenv("MINZ_Z3_VM"), "-H", filepath.Join(d, "zvm.nanz"))
		if zorkIIGarden {
			for line, command := range bytes.Split(route, []byte{10}) {
				if len(command) > 79 {
					t.Fatalf("Spectrum route line %d exceeds 79 input characters (%d)", line+1, len(command))
				}
			}
		}

		vm.Stdin = bytes.NewReader(route)
		reference, err = vm.Output()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(reference, []byte("Z3 error")) {
			t.Fatalf("reference failed%q", reference)
		}
		if tool := os.Getenv("MINZ_Z3_FROTZ"); tool != "" && !restart && !saveRestore && !earlyDungeon && !zorkIIGarden {
			referenceCmd := exec.Command(tool, "-m", "-w", "255", externalStory)
			referenceCmd.Stdin = bytes.NewReader(route)
			external, err := referenceCmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			normalize := func(data []byte) string {
				var lines []string
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimRight(line, " \r")
					line = strings.TrimLeft(line, "> ")
					if line == "" || strings.Contains(line, "Score:") && strings.Contains(line, "Moves:") {
						continue
					}
					lines = append(lines, line)
				}
				// Ignore terminal wrapping while retaining every word and punctuation.
				return strings.Join(strings.Fields(strings.Join(lines, "\n")), " ")
			}
			if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(d, "explore-frotz.txt"), external, 0644); err != nil {
					t.Fatal(err)
				}
			}
			normalizedReference := reference
			if compressedRestart {
				// Host/native interpreter prints its own restart notice;
				// Frotz does not. This is the only removed non-layout text.
				normalizedReference = bytes.ReplaceAll(reference, []byte("Restarting.\n"), []byte("\n"))
				// Frotz may append a status row to an existing prompt line.
				// Remove only the row, retaining the preceding game prompt.
				external = regexp.MustCompile(`Inside the Barrow[ ]+Score: [0-9]+[ ]+Moves: [0-9]+`).ReplaceAll(external, nil)
			}
			if treasure {
				// Forest ambience is random across interpreters. Remove only
				// this exact optional sentence; score/objects/all other text remain.
				ambient := []byte("You hear in the distance the chirping of a song bird.\n")
				if bytes.Count(normalizedReference, ambient) > 1 || bytes.Count(external, ambient) > 1 {
					t.Fatal("unexpected repeated ambience")
				}
				normalizedReference = bytes.ReplaceAll(normalizedReference, ambient, nil)
				external = bytes.ReplaceAll(external, ambient, nil)
			}
			if normalize(normalizedReference) != normalize(external) {
				t.Fatalf("VM/Frotz exploration differs VM%q Frotz%q", reference, external)
			}
			t.Log("exploration VM text matches independently implemented Frotz")
		}
		if earlyDungeon && !lateSave && os.Getenv("MINZ_Z3_FROTZ") != "" {
			refSeed := "12"
			if trident || treasureRoom || coldPassage || fullWalkthrough {
				refSeed = "1"
			}
			ref := exec.Command(os.Getenv("MINZ_Z3_FROTZ"), "-m", "-w", "255", "-s", refSeed, externalStory)
			ref.Stdin = bytes.NewReader(route)
			output, err := ref.Output()
			if err != nil {
				t.Fatal(err)
			}
			checkpoint := []string{"Your score is 63 (total of 350 points), in 57 moves.", "You are carrying:\n  A sceptre\n  A brass bell\n  A black book\n  A pair of candles\n  A torch (providing light)\n  A nasty knife\n  A brass lantern\n  A clove of garlic"}
			if damLobby {
				checkpoint = []string{"Your score is 102 (total of 350 points), in 104 moves.", "You are carrying:\n  A platinum bar\n  A pair of candles\n  A brass bell\n  A black book\n  A brass lantern\n  A torch (providing light)"}
			}
			if crystalSkull {
				checkpoint = []string{"Your score is 127 (total of 350 points), in 134 moves.", "Your collection of treasures consists of:\n  A platinum bar\n  A crystal skull\n  A gold coffin", "You are carrying:\n  A brass lantern\n  A torch (providing light)"}
			}
			if cyclops {
				checkpoint = []string{"Your score is 142 (total of 350 points), in 150 moves.", "Your collection of treasures consists of:\n  A leather bag of coins\n  A platinum bar\n  A crystal skull\n  A gold coffin", "You are carrying:\n  A rusty knife\n  A brass lantern\n  A torch (providing light)"}
			}
			if trident {
				checkpoint = []string{"Your score is 146 (total of 350 points), in 204 moves.", "Your collection of treasures consists of:\n  A leather bag of coins\n  A platinum bar\n  A crystal skull\n  A gold coffin", "You are carrying:\n  A hand-held air pump\n  A crystal trident\n  A brass lantern\n  A torch (providing light)"}
			}
			if treasureRoom {
				checkpoint = []string{"Your score is 233 (total of 350 points), in 264 moves.", "There is a silver chalice, intricately engraved, here.", "You are carrying:\n  A nasty knife\n  A rusty knife\n  A brass lantern\n  A torch (providing light)"}
			}
			if coldPassage {
				checkpoint = []string{"Your score is 271 (total of 350 points), in 304 moves.", "Cold Passage", "You are carrying:\n  A clove of garlic\n  A screwdriver\n  A brass lantern\n  A torch (providing light)"}
			}
			if fullWalkthrough {
				checkpoint = []string{"Inside the Barrow", "You have mastered the first part of the ZORK trilogy.", "Your score is 350 (total of 350 points), in 399 moves.", "This gives you the rank of Master Adventurer."}
			}
			for _, want := range checkpoint {
				if !bytes.Contains(output, []byte(want)) {
					t.Fatalf("Frotz checkpoint missing %q", want)
				}
			}
			if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(d, "dungeon-frotz.txt"), output, 0644); err != nil {
					t.Fatal(err)
				}
			}
			t.Log("independent Frotz confirms score and complete inventory; random combat text is not compared")
		}
		if zorkIIGarden && os.Getenv("MINZ_Z3_FROTZ") != "" {
			seed := "1"
			if zorkIIRiddle {
				seed = "2"
			}
			if zorkIIWell {
				seed = "1"
			}
			if zorkIIKey {
				seed = "14"
			}
			ref := exec.Command(os.Getenv("MINZ_Z3_FROTZ"), "-m", "-w", "255", "-s", seed, externalStory)
			ref.Stdin = bytes.NewReader(route)
			output, err := ref.Output()
			if err != nil {
				t.Fatal(err)
			}
			wants := []string{"Gazebo", "A newspaper\n  A matchbook\n  A letter opener\n  A place mat\n  A china teapot\n  A elvish sword\n  A lamp (providing light)", "Your score would be 0 (total of 400 points), in 15 moves."}
			if zorkIIRiddle {
				wants = []string{"stone door quietly swings open", "A pearl necklace\n  A newspaper\n  A matchbook\n  A letter opener\n  A place mat\n  A china teapot\n  A elvish sword\n  A lamp (providing light)", "Your score would be 20 (total of 400 points), in 25 moves."}
			}
			if zorkIIWell {
				wants = []string{"The bucket rises and comes to a stop.", "Tea Room", "A pearl necklace\n  A newspaper\n  A matchbook\n  A letter opener\n  A place mat\n  A china teapot\n  A elvish sword\n  A lamp (providing light)", "Your score would be 30 (total of 400 points), in 36 moves."}
			}
			if zorkIICakes {
				wants = []string{"Suddenly, the room appears to have become very large", "Most of the pool evaporates", "The room around you seems to be getting smaller.", "You are carrying:\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 45 (total of 400 points), in 44 moves."}
			}
			if zorkIIRobot {
				wants = []string{"The cage shakes and is hurled across the room.", "You are carrying:\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 65 (total of 400 points), in 56 moves.", "Low Room"}
			}
			if zorkIIReturnWell {
				wants = []string{"The cage shakes and is hurled across the room.", "The bucket descends and comes to a stop.", "Circular Room", "You are carrying:\n  A china teapot\n  The china teapot contains:\n    A quantity of water\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 65 (total of 400 points), in 63 moves."}
			}

			if zorkIIKey {
				wants = []string{"The place mat fits easily under the door.", "As the place mat is moved, a rusty iron key falls from it and onto the floor.", "The door is now unlocked.", "Dreary Room", "You are carrying:\n  A blue crystal sphere\n  A letter opener\n  A rusty iron key\n  A china teapot\n  The china teapot contains:\n    A quantity of water\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 85 (total of 400 points), in 86 moves."}
			}

			if zorkIIDragon {
				wants = []string{"the melting of the ice has revealed a passage leading west.", "Thank you for rescuing me from that horrid worm", "You are carrying:\n  A golden dragon statuette\n  A blue crystal sphere\n  A letter opener\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 110 (total of 400 points), in 105 moves."}
			}

			if zorkIIPrincess {
				wants = []string{"The princess presses a loose piece of marble", "The princess enters the gazebo.", "Shyly, a unicorn peeks out of the hedges.", "You are carrying:\n  A delicate gold key\n  A perfect rose\n  A blue crystal sphere\n  A letter opener\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 105 (total of 400 points), in 137 moves."}
			}

			if zorkIIPrincessTreasure {
				wants = []string{"The princess presses a loose piece of marble", "Shyly, a unicorn peeks out of the hedges.", "You are carrying:\n  A golden dragon statuette\n  A delicate gold key\n  A perfect rose\n  A blue crystal sphere\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 125 (total of 400 points), in 149 moves."}
			}

			if zorkIIBank {
				wants = []string{"Safety Depository", "Vault", "You are carrying:\n  A stack of zorkmid bills\n  A portrait of J. Pierpont Flathead\n  A lamp (providing light)", "Your score would be 170 (total of 400 points), in 170 moves."}
			}

			for _, want := range wants {
				if !bytes.Contains(output, []byte(want)) {
					t.Fatalf("Frotz garden checkpoint missing %q", want)
				}
			}
			if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(d, "garden-frotz.txt"), output, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		keyText = bytes.ReplaceAll(route, []byte{10}, []byte{13})
	}
	keys := []byte{0, 'z', 'x', 'c', 'v', 'a', 's', 'd', 'f', 'g', 'q', 'w', 'e', 'r', 't', '1', '2', '3', '4', '5', '0', '9', '8', '7', '6', 'p', 'o', 'i', 'u', 'y', 13, 'l', 'k', 'j', 'h', 32, 0, 'm', 'n', 'b'}
	keyAt := 0
	ready := false
	minSP := uint16(0xbf00)
	var rendered []byte
	var observedSaveLengths []int
	inputEcho := false
	var inputReturn, inputSP uint16

	limit := 200000000
	if earlyDungeon || zorkIIRiddle {
		limit = 2000000000
	}
	if fullWalkthrough {
		limit = 3000000000
	}
	for i := 0; i < limit; i++ {

		if console && !keyboard && m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
			break
		}
		if m.CPU.PC() == 0xbf10 && m.CPU.SP() == 0xbf02 {
			break
		}
		if saveRestore && m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
			addr := uint16(a.Symbols["native_save_length"])
			length := int(m.Memory.Read(addr)) | int(m.Memory.Read(addr+1))<<8
			if length > 0 && (len(observedSaveLengths) == 0 || observedSaveLengths[len(observedSaveLengths)-1] != length) {
				observedSaveLengths = append(observedSaveLengths, length)
			}
		}
		if keyboard {
			if m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
				inputEcho = true
				inputSP = m.CPU.SP()
				inputReturn = uint16(m.Memory.Read(inputSP)) | uint16(m.Memory.Read(inputSP+1))<<8
			}
			if inputEcho && m.CPU.PC() == inputReturn && m.CPU.SP() == inputSP+2 {
				inputEcho = false
			}
			outputSymbol := a.Symbols["native_putchar"]
			if windows {
				outputSymbol = a.Symbols["zx__console__z3_window_put"]
			}
			if !inputEcho && m.CPU.PC() == uint16(outputSymbol) {
				rendered = append(rendered, byte(m.CPU.AF()>>8))
			}
			if m.CPU.PC() == uint16(a.Symbols["zx__console__con_getchar"]) {
				m.Keyboard.Reset()
				ready = true
			}
			if ready && m.CPU.PC() == uint16(a.Symbols["zx__console__scan_row_key"]) {
				if keyAt == len(keyText) {
					t.Fatal("unexpected additional keyboard input")
				}
				found := false
				for idx, ch := range keys {
					if ch == keyText[keyAt] {
						m.Keyboard.KeyPress(idx/5, idx%5)
						found = true
						break
					}
				}
				if !found && keyText[keyAt] == '"' {
					m.Keyboard.KeyPress(spectrum.KeySym.Row, spectrum.KeySym.Bit)
					m.Keyboard.KeyPress(spectrum.KeyP.Row, spectrum.KeyP.Bit)
					found = true
				}
				if !found && keyText[keyAt] == ',' {
					m.Keyboard.KeyPress(spectrum.KeySym.Row, spectrum.KeySym.Bit)
					m.Keyboard.KeyPress(spectrum.KeyN.Row, spectrum.KeyN.Bit)
					found = true
				}
				if !found {
					t.Fatalf("unmapped key %q", keyText[keyAt])
				}
				keyAt++
				ready = false
			}
		}
		m.CPU.DoOpcode()
		if m.CPU.SP() < minSP {
			minSP = m.CPU.SP()
		}

	}
	if keyboard {
		if m.CPU.PC() != 0xbf10 || m.CPU.SP() != 0xbf02 || keyAt != len(keyText) || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || minSP < 0xbe00 {
			t.Fatalf("keyboard keys%d/%d PC/SP%04x/%04x minSP%04x", keyAt, len(keyText), m.CPU.PC(), m.CPU.SP(), minSP)
		}
		if explore {
			if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"explore-native.txt": rendered, "explore-vm.txt": reference} {
					if err := os.WriteFile(filepath.Join(d, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !bytes.Equal(rendered, reference) {
				at := 0
				for at < len(rendered) && at < len(reference) && rendered[at] == reference[at] {
					at++
				}
				lo := at - 100
				if lo < 0 {
					lo = 0
				}
				nativeEnd, refEnd := at+300, at+300
				if nativeEnd > len(rendered) {
					nativeEnd = len(rendered)
				}
				if refEnd > len(reference) {
					refEnd = len(reference)
				}
				t.Fatalf("exploration first differs at byte %d (native %d bytes, reference %d): native %q reference %q", at, len(rendered), len(reference), rendered[lo:nativeEnd], reference[lo:refEnd])
			}
			phrases := []string{"Kitchen", "Living Room", "Cellar", "A brass lantern", "A sword"}
			if treasure {
				phrases = []string{"Up a Tree", "Taken.", "Done.", "Your collection of treasures consists of:\n  A jewel-encrusted egg", "Your score is 20 (total of 350 points), in 13 moves.", "You are empty-handed."}
			}
			if earlyDungeon {
				phrases = []string{"The unconscious troll cannot defend himself: He dies.", "Your score is 63 (total of 350 points), in 57 moves.", "You are carrying:\n  A sceptre\n  A brass bell\n  A black book\n  A pair of candles\n  A torch (providing light)", "Forest Path"}
			}
			if damLobby {
				phrases = []string{"Dam Lobby", "Your score is 102 (total of 350 points), in 104 moves.", "You are carrying:\n  A platinum bar\n  A pair of candles\n  A brass bell\n  A black book\n  A brass lantern\n  A torch (providing light)"}
			}
			if crystalSkull {
				phrases = []string{"Land of the Dead", "Your score is 127 (total of 350 points), in 134 moves.", "Your collection of treasures consists of:\n  A platinum bar\n  A crystal skull\n  A gold coffin", "You are carrying:\n  A brass lantern\n  A torch (providing light)"}
			}
			if cyclops {
				phrases = []string{"The cyclops, hearing the name of his father's deadly nemesis, flees the room", "Your score is 142 (total of 350 points), in 150 moves.", "Your collection of treasures consists of:\n  A leather bag of coins\n  A platinum bar\n  A crystal skull\n  A gold coffin", "You are carrying:\n  A rusty knife\n  A brass lantern\n  A torch (providing light)"}
			}
			if trident {
				phrases = []string{"Atlantis Room", "Your score is 146 (total of 350 points), in 204 moves.", "Your collection of treasures consists of:\n  A leather bag of coins\n  A platinum bar\n  A crystal skull\n  A gold coffin", "You are carrying:\n  A hand-held air pump\n  A crystal trident\n  A brass lantern\n  A torch (providing light)"}
			}
			if treasureRoom {
				phrases = []string{"Treasure Room", "Your score is 233 (total of 350 points), in 264 moves.", "There is a silver chalice, intricately engraved, here.", "You are carrying:\n  A nasty knife\n  A rusty knife\n  A brass lantern\n  A torch (providing light)"}
			}
			if coldPassage {
				phrases = []string{"Your score is 271 (total of 350 points), in 304 moves.", "Cold Passage", "You are carrying:\n  A clove of garlic\n  A screwdriver\n  A brass lantern\n  A torch (providing light)"}
			}
			if fullWalkthrough {
				phrases = []string{"Inside the Barrow", "You have mastered the first part of the ZORK trilogy.", "Your score is 350 (total of 350 points), in 399 moves.", "This gives you the rank of Master Adventurer."}
			}
			if zorkII {
				phrases = []string{"ZORK II: The Wizard of Frobozz", "Inside the Barrow", "You are empty-handed.", "Taken.", "The lamp is now on.", "Narrow Tunnel", "total of 400 points"}
			}
			if compressedRestart {
				phrases = []string{"Inside the Barrow", "The lamp is now on.", "Narrow Tunnel", "You are empty-handed."}
			}
			if saveRestore {
				phrases = []string{"Ok.", "A leaflet", "You are empty-handed."}
				if cellarSave {
					phrases = []string{"Cellar", "A sword", "A brass lantern", "Dropped."}
				}
			}
			if treasureSave {
				phrases = []string{"Up a Tree", "Ok.", "Your collection of treasures consists of:\n  A jewel-encrusted egg", "Your score is 20 (total of 350 points)", "You are empty-handed."}
			}
			if compressedSave {
				phrases = []string{"Narrow Tunnel", "Ok.", "Dropped.", "You are carrying:"}
			}
			if restart {
				phrases = []string{"You are empty-handed.", "West of House"}
			}
			if lateSave {
				phrases = []string{"Inside the Barrow", "You have mastered the first part of the ZORK trilogy.", "Your score is 350 (total of 350 points), in 400 moves.", "Master Adventurer", "Ok.", "Dropped."}
				inventory := []byte("You are carrying:\n  A clove of garlic\n  A screwdriver\n  A brass lantern\n  A torch (providing light)")
				if bytes.Count(rendered, inventory) != 2 {
					t.Fatal("late restore did not recover complete inventory", string(rendered))
				}
				if len(observedSaveLengths) != 1 || observedSaveLengths[0] > 2204 {
					t.Fatal("late save missing/oversized", observedSaveLengths)
				}
				t.Logf("late-game native SAVE %d/2204 bytes", observedSaveLengths[0])
			}
			if zorkIIGarden {
				phrases = []string{"Gazebo", "Path Near Stream", "You are carrying:\n  A newspaper\n  A matchbook\n  A letter opener\n  A place mat\n  A china teapot\n  A elvish sword\n  A lamp (providing light)", "Your score would be 0 (total of 400 points), in 15 moves."}
			}
			if zorkIIRiddle {
				phrases = []string{"stone door quietly swings open", "Pearl Room", "A pearl necklace\n  A newspaper\n  A matchbook\n  A letter opener\n  A place mat\n  A china teapot\n  A elvish sword\n  A lamp (providing light)", "Your score would be 20 (total of 400 points), in 25 moves."}
			}
			if zorkIIWell {
				phrases = []string{"The bucket rises and comes to a stop.", "Tea Room", "A pearl necklace\n  A newspaper\n  A matchbook\n  A letter opener\n  A place mat\n  A china teapot\n  A elvish sword\n  A lamp (providing light)", "Your score would be 30 (total of 400 points), in 36 moves."}
			}
			if zorkIICakes {
				phrases = []string{"Suddenly, the room appears to have become very large", "Most of the pool evaporates", "The room around you seems to be getting smaller.", "You are carrying:\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 45 (total of 400 points), in 44 moves."}
			}
			if zorkIIRobot {
				phrases = []string{"The cage shakes and is hurled across the room.", "You are carrying:\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 65 (total of 400 points), in 56 moves.", "Low Room"}
			}
			if zorkIIReturnWell {
				phrases = []string{"The cage shakes and is hurled across the room.", "The bucket descends and comes to a stop.", "Circular Room", "You are carrying:\n  A china teapot\n  The china teapot contains:\n    A quantity of water\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 65 (total of 400 points), in 63 moves."}
			}

			if zorkIIKey {
				phrases = []string{"The place mat fits easily under the door.", "As the place mat is moved, a rusty iron key falls from it and onto the floor.", "The door is now unlocked.", "Dreary Room", "You are carrying:\n  A blue crystal sphere\n  A letter opener\n  A rusty iron key\n  A china teapot\n  The china teapot contains:\n    A quantity of water\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 85 (total of 400 points), in 86 moves."}
			}

			if zorkIIDragon {
				phrases = []string{"the melting of the ice has revealed a passage leading west.", "Thank you for rescuing me from that horrid worm", "You are carrying:\n  A golden dragon statuette\n  A blue crystal sphere\n  A letter opener\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 110 (total of 400 points), in 105 moves."}
			}

			if zorkIIPrincess {
				phrases = []string{"The princess presses a loose piece of marble", "The princess enters the gazebo.", "Shyly, a unicorn peeks out of the hedges.", "You are carrying:\n  A delicate gold key\n  A perfect rose\n  A blue crystal sphere\n  A letter opener\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 105 (total of 400 points), in 139 moves."}
			}

			if zorkIIPrincessTreasure {
				phrases = []string{"The princess presses a loose piece of marble", "Shyly, a unicorn peeks out of the hedges.", "You are carrying:\n  A golden dragon statuette\n  A delicate gold key\n  A perfect rose\n  A blue crystal sphere\n  A red crystal sphere\n  A package of candy\n  A pearl necklace\n  A lamp (providing light)", "Your score would be 125 (total of 400 points), in 151 moves."}
			}

			if zorkIIBank {
				phrases = []string{"Safety Depository", "Vault", "You are carrying:\n  A stack of zorkmid bills\n  A portrait of J. Pierpont Flathead\n  A lamp (providing light)", "Your score would be 170 (total of 400 points), in 172 moves."}
			}

			for _, phrase := range phrases {
				if !bytes.Contains(rendered, []byte(phrase)) {
					t.Fatalf("exploration missing %q: %q", phrase, rendered)
				}
			}
			if treasureSave {
				collection := []byte("Your collection of treasures consists of:\n  A jewel-encrusted egg")
				emptyCase := bytes.Index(rendered, []byte("The trophy case is empty."))
				carrying := bytes.Index(rendered, []byte("You are carrying:\n  A jewel-encrusted egg"))
				restoredCase := bytes.Index(rendered, collection)
				emptyInventory := bytes.Index(rendered, []byte("You are empty-handed."))
				if bytes.Count(rendered, collection) != 1 || emptyCase < 0 || carrying <= emptyCase || restoredCase <= carrying || emptyInventory <= restoredCase {
					t.Fatal("treasure ownership not restored", string(rendered))
				}
				if len(observedSaveLengths) != 1 || observedSaveLengths[0] > 2204 {
					t.Fatal("missing/oversized treasure save", observedSaveLengths)
				}
				if !bytes.Contains(rendered, []byte("Your score is 20 (total of 350 points)")) {
					t.Fatal("treasure score lost")
				}
				dynamic := int(story[14])<<8 | int(story[15])
				for at := dynamic; at < len(story); at++ {
					if m.Memory.RAM[pages[at/16384]][at%16384] != story[at] {
						t.Fatalf("static story changed at %d", at)
					}
				}
				for offset, b := range restartArchive {
					at := len(story) + offset
					if m.Memory.RAM[pages[at/16384]][at%16384] != b {
						t.Fatalf("restart archive changed at %d", at)
					}
				}
				t.Logf("treasure save %d bytes; case ownership, inventory and score restored", observedSaveLengths[0])
			}
			if cellarSave {
				restored := []byte("You are carrying:\n  A sword\n  A brass lantern (providing light)")
				if bytes.Count(rendered, restored) != 2 {
					t.Fatal("both restores must recover sword and lit lamp", string(rendered))
				}
				if len(observedSaveLengths) != 1 || observedSaveLengths[0] > 2204 {
					t.Fatal("missing or oversized native save", observedSaveLengths)
				}
				t.Logf("cellar native save %d bytes; both inventory restores independently checked", observedSaveLengths[0])
			}
			if windowCellar {
				if !bytes.Contains(rendered, []byte("You are empty-handed.")) || bytes.Count(rendered, []byte("West of House")) < 2 {
					t.Fatal("restart did not reset world")
				}
				dynamic := int(story[14])<<8 | int(story[15])
				for at := dynamic; at < len(story); at++ {
					if m.Memory.RAM[pages[at/16384]][at%16384] != story[at] {
						t.Fatalf("static story changed at %d", at)
					}
				}
				for offset, b := range restartArchive {
					at := len(story) + offset
					if m.Memory.RAM[pages[at/16384]][at%16384] != b {
						t.Fatalf("restart archive changed at %d", at)
					}
				}
				if m.Memory.RAM[0][1]&48 != 32 {
					t.Fatal("restart lost UI capability")
				}
			}
			if compressedSave {
				if !bytes.Contains(rendered, []byte("You are carrying:\n  A lamp (providing light)\n")) || !bytes.Contains(rendered, []byte("You are carrying:\n  A elvish sword\n  A lamp (providing light)")) {
					t.Fatal("restore must recover sword while preserving lit lamp")
				}
				if len(observedSaveLengths) != 1 || observedSaveLengths[0] > 700 {
					t.Fatal("missing/oversized save", observedSaveLengths)
				}
				t.Logf("compressed save record %d bytes", observedSaveLengths[0])
			}
			if fullWalkthrough {
				dynamic := int(story[14])<<8 | int(story[15])
				for at := dynamic; at < len(story); at++ {
					if m.Memory.RAM[pages[at/16384]][at%16384] != story[at] {
						t.Fatalf("static story modified at %d", at)
					}
				}
				for off, b := range restartArchive {
					at := len(story) + off
					if m.Memory.RAM[pages[at/16384]][at%16384] != b {
						t.Fatalf("restart archive modified at %d", at)
					}
				}
			}
			if zorkIIWindows && m.Memory.Read(uint16(a.Symbols["story_verified"])) != 1 {
				t.Fatal("SAVE/RESTORE/RESTART lost original checksum result")
			}
			if compressedStory {
				dynamic := int(story[14])<<8 | int(story[15])
				for i, b := range storageImage {
					if i < dynamic || compressedSave && i >= 5*16384+15684 {
						continue
					}
					if m.Memory.RAM[pages[i/16384]][i%16384] != b {
						t.Fatalf("compressed immutable storage changed at %d", i)
					}
				}
				if compressedRestart {
					if bytes.Count(rendered, []byte("ZORK II: The Wizard of Frobozz")) != 2 || !bytes.Contains(rendered, []byte("You are empty-handed.")) {
						t.Fatal("restart did not return original world")
					}
				}
				at := uint16(a.Symbols["native_story_cache_misses"])
				misses := int(m.Memory.Read(at)) | int(m.Memory.Read(at+1))<<8
				t.Logf("compressed resident %d bytes; cache misses %d (u16 counter)", len(a.Binary), misses)
			}
			t.Logf("exploration %d keys; %d output bytes; %d T; native stack %d bytes", keyAt, len(rendered), m.CPU.Tstates(), 0xbf00-int(minSP))
			return
		}
		for _, phrase := range []string{"Opening the small mailbox reveals a leaflet.", "ZORK is a game of adventure, danger, and low cunning.", "You are carrying:\n  A leaflet", "Your score is 0 (total of 350 points), in 5 moves."} {
			if !bytes.Contains(rendered, []byte(phrase)) {
				t.Fatalf("keyboard missing%q output%q", phrase, rendered)
			}
		}
		if bytes.Contains(rendered, []byte("Z3 error")) {
			t.Fatalf("keyboard error%q", rendered)
		}
		t.Logf("keyboard %d keys; %d bytes; %d T; native stack %d bytes", keyAt, len(a.Binary), m.CPU.Tstates(), 0xbf00-int(minSP))
		return
	}

	if console {
		if m.CPU.PC() != uint16(a.Symbols["tui_read_line_or_eof"]) {
			t.Fatalf("console did not reach input PC%04x", m.CPU.PC())
		}
		// Independent 32x24 terminal model including wrap and scrolling.
		cells := [24][32]byte{}
		row, col := 0, 0
		newline := func() {
			col = 0
			row++
			if row == 24 {
				for r := 0; r < 23; r++ {
					cells[r] = cells[r+1]
				}
				cells[23] = [32]byte{}
				row = 23
			}
		}
		text := "ZORK I: The Great Underground Empire\nInfocom interactive fiction - a fantasy story\nCopyright (c) 1981, 1982, 1983, 1984, 1985, 1986 Infocom, Inc. All rights reserved.\nZORK is a registered trademark of Infocom, Inc.\nRelease 1 / Serial number 000001\n\nWest of House\nYou are standing in an open field west of a white house, with a boarded front door.\nThere is a small mailbox here.\n\n>"
		for _, ch := range []byte(text) {
			if ch == 10 || ch == 13 {
				newline()
				continue
			}
			cells[row][col] = ch
			col++
			if col == 32 {
				newline()
			}
		}
		for r := 0; r < 24; r++ {
			for c := 0; c < 32; c++ {
				ch := cells[r][c]
				for l := 0; l < 8; l++ {
					want := byte(0)
					if ch >= 32 {
						want = byte((int(ch)*17 + l*29) ^ 0x5a)
					}
					addr := uint16(0x4000 | ((r & 0x18) << 8) | (l << 8) | ((r & 7) << 5) | c)
					if m.Memory.Read(addr) != want {
						t.Fatalf("screen r%d c%d l%d got%02x want%02x", r, c, l, m.Memory.Read(addr), want)
					}
				}
				if m.Memory.Read(uint16(0x5800+r*32+c)) != 7 {
					t.Fatal("attribute mismatch")
				}
			}
		}
		if m.Memory.Read(uint16(a.Symbols["zx__console__con_col"])) != byte(col) || m.Memory.Read(uint16(a.Symbols["zx__console__con_row"])) != byte(row) || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() {
			t.Fatal("console cursor/paging/IFF mismatch")
		}
		t.Logf("console startup %d bytes; %d T; SP%04x cursor%d/%d", len(a.Binary), m.CPU.Tstates(), m.CPU.SP(), row, col)
		return
	}
	if m.CPU.PC() != 0xbf10 || m.CPU.SP() != 0xbf02 {
		t.Fatalf("PC/SP=%04x/%04x vmPC=%02x%02x%02x steps=%02x%02x", m.CPU.PC(), m.CPU.SP(), m.Memory.Read(uint16(a.Symbols["pc"])+2), m.Memory.Read(uint16(a.Symbols["pc"])+1), m.Memory.Read(uint16(a.Symbols["pc"])), m.Memory.Read(uint16(a.Symbols["steps"])+1), m.Memory.Read(uint16(a.Symbols["steps"])))
	}
	if m.CPU.IFF1() || m.CPU.IFF2() || m.Memory.PagingState() != 0x10 {
		t.Fatal("native boundary state lost")
	}
	lengthAt := uint16(a.Symbols["native_output_length"])
	length := int(m.Memory.Read(lengthAt)) | int(m.Memory.Read(lengthAt+1))<<8
	if length > 1024 {
		t.Fatal("invalid capture length", length)
	}
	got := make([]byte, length)
	for i := range got {
		got[i] = m.Memory.Read(uint16(a.Symbols["native_output"]) + uint16(i))
	}
	wantPath := "stream3.out"
	if highMemory {
		wantPath = "high_memory.out"
	}
	want, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures", wantPath))
	if err != nil {
		t.Fatal(err)
	}
	if !highMemory && externalStory == "" {
		for _, expected := range []struct {
			address int
			data    []byte
		}{
			{0x492, []byte{0, 3, 65, 13, 66}}, {0x4b2, []byte{0, 2, 88, 90}}, {0x4d2, []byte{0, 1, 89}},
		} {
			if !bytes.Equal(m.Memory.RAM[0][expected.address:expected.address+len(expected.data)], expected.data) {
				t.Fatalf("stream RAM mismatch at %04x", expected.address)
			}
		}
	}
	if externalStory != "" {
		for _, phrase := range []string{"ZORK I: The Great Underground Empire", "West of House", "There is a small mailbox here."} {
			if !bytes.Contains(got, []byte(phrase)) {
				t.Fatalf("missing %q: output %q", phrase, got)
			}
		}
		if commands != "" {
			at := uint16(a.Symbols["native_command_at"])
			consumed := int(m.Memory.Read(at)) | int(m.Memory.Read(at+1))<<8
			if consumed != len(commands) {
				t.Fatalf("consumed %d of %d command bytes", consumed, len(commands))
			}
			for _, phrase := range []string{"Opening the small mailbox reveals a leaflet.", "ZORK is a game of adventure, danger, and low cunning.", "You are carrying:\n  A leaflet", "Your score is 0 (total of 350 points), in 5 moves.", "Do you wish to leave the game?"} {
				if !bytes.Contains(got, []byte(phrase)) {
					t.Fatalf("gameplay missing %q: %q", phrase, got)
				}
			}
			if bytes.Count(got, []byte("There is a small mailbox here.")) != 2 {
				t.Fatalf("LOOK did not repeat room: %q", got)
			}
		}
		if bytes.Contains(got, []byte("Z3 error")) {
			t.Fatalf("interpreter failure %q", got)
		}
	} else if !bytes.Equal(got, want) {
		t.Fatalf("output got %q want %q; vmPC=%02x%02x%02x steps=%d", got, want, m.Memory.Read(uint16(a.Symbols["pc"])+2), m.Memory.Read(uint16(a.Symbols["pc"])+1), m.Memory.Read(uint16(a.Symbols["pc"])), int(m.Memory.Read(uint16(a.Symbols["steps"]))))
	}
	t.Logf("native interpreter %d bytes; %d T; output %q", len(a.Binary), m.CPU.Tstates(), got)
}
