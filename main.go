package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Linux input event constants
const (
	UINPUT_MAX_NAME_SIZE = 80
	
	// Event types
	EV_SYN = 0x00
	EV_KEY = 0x01
	EV_REL = 0x02
	
	// Key codes mapping
	KEY_ESC          = 1
	KEY_1            = 2
	KEY_2            = 3
	KEY_3            = 4
	KEY_4            = 5
	KEY_5            = 6
	KEY_6            = 7
	KEY_7            = 8
	KEY_8            = 9
	KEY_9            = 10
	KEY_0            = 11
	KEY_MINUS        = 12
	KEY_EQUAL        = 13
	KEY_BACKSPACE    = 14
	KEY_TAB          = 15
	KEY_Q            = 16
	KEY_W            = 17
	KEY_E            = 18
	KEY_R            = 19
	KEY_T            = 20
	KEY_Y            = 21
	KEY_U            = 22
	KEY_I            = 23
	KEY_O            = 24
	KEY_P            = 25
	KEY_LEFTBRACE    = 26
	KEY_RIGHTBRACE   = 27
	KEY_ENTER        = 28
	KEY_LEFTCTRL     = 29
	KEY_A            = 30
	KEY_S            = 31
	KEY_D            = 32
	KEY_F            = 33
	KEY_G            = 34
	KEY_H            = 35
	KEY_J            = 36
	KEY_K            = 37
	KEY_L            = 38
	KEY_SEMICOLON    = 39
	KEY_APOSTROPHE   = 40
	KEY_GRAVE        = 41
	KEY_LEFTSHIFT    = 42
	KEY_BACKSLASH    = 43
	KEY_Z            = 44
	KEY_X            = 45
	KEY_C            = 46
	KEY_V            = 47
	KEY_B            = 48
	KEY_N            = 49
	KEY_M            = 50
	KEY_COMMA        = 51
	KEY_DOT          = 52
	KEY_SLASH        = 53
	KEY_RIGHTSHIFT   = 54
	KEY_KPASTERISK   = 55
	KEY_LEFTALT      = 56
	KEY_SPACE        = 57
	KEY_CAPSLOCK     = 58
	KEY_F1           = 59
	KEY_F2           = 60
	KEY_F3           = 61
	KEY_F4           = 62
	KEY_F5           = 63
	KEY_F6           = 64
	KEY_F7           = 65
	KEY_F8           = 66
	KEY_F9           = 67
	KEY_F10          = 68
	KEY_F11          = 87
	KEY_F12          = 88
	KEY_UP           = 103
	KEY_DOWN         = 108
	KEY_LEFT         = 105
	KEY_RIGHT        = 106
	KEY_LEFTMETA     = 125  // Super/Windows key
	KEY_RIGHTMETA    = 126
	KEY_RIGHTCTRL    = 97
	KEY_RIGHTALT     = 100
	KEY_DELETE       = 111
	KEY_HOME         = 102
	KEY_END          = 107
	KEY_PAGEUP       = 104
	KEY_PAGEDOWN     = 109
	KEY_INSERT       = 110
	
	// Mouse buttons
	BTN_LEFT   = 0x110
	BTN_RIGHT  = 0x111
	BTN_MIDDLE = 0x112
	REL_X      = 0x00
	REL_Y      = 0x01
	REL_WHEEL  = 0x08
	
	// IOCTLs
	UI_SET_EVBIT  = 0x40045564
	UI_SET_KEYBIT = 0x40045565
	UI_SET_RELBIT = 0x40045567
	UI_DEV_SETUP  = 0x405c5503
	UI_DEV_CREATE = 0x5501
	UI_DEV_DESTROY = 0x5502
)

// Input event structure
type InputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// UInput device setup structure
type UInputSetup struct {
	ID   InputID
	Name [UINPUT_MAX_NAME_SIZE]byte
	FFEffectsMax uint32
}

type InputID struct {
	Bustype uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

type VirtualInputDevice struct {
	fd int
	keyMap map[string]uint16
}

// HTTP request structures
type KeyRequest struct {
	Keys []string `json:"keys"`
}

type MouseRequest struct {
	X int32 `json:"x"`
	Y int32 `json:"y"`
}

type MouseClickRequest struct {
	Button string `json:"button"` // "left", "right", "middle"
}

type MouseScrollRequest struct {
	Direction int32 `json:"direction"` // positive = up, negative = down
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewVirtualInputDevice() (*VirtualInputDevice, error) {
	// Open uinput device
	fd, err := syscall.Open("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/uinput: %v", err)
	}

	// Enable key events
	if err := ioctl(fd, UI_SET_EVBIT, EV_KEY); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to enable key events: %v", err)
	}

	// Enable relative events (for mouse)
	if err := ioctl(fd, UI_SET_EVBIT, EV_REL); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to enable relative events: %v", err)
	}

	// Enable sync events
	if err := ioctl(fd, UI_SET_EVBIT, EV_SYN); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to enable sync events: %v", err)
	}

	// Enable all keyboard keys
	keys := []uint16{
		KEY_ESC, KEY_1, KEY_2, KEY_3, KEY_4, KEY_5, KEY_6, KEY_7, KEY_8, KEY_9, KEY_0,
		KEY_MINUS, KEY_EQUAL, KEY_BACKSPACE, KEY_TAB,
		KEY_Q, KEY_W, KEY_E, KEY_R, KEY_T, KEY_Y, KEY_U, KEY_I, KEY_O, KEY_P,
		KEY_LEFTBRACE, KEY_RIGHTBRACE, KEY_ENTER, KEY_LEFTCTRL,
		KEY_A, KEY_S, KEY_D, KEY_F, KEY_G, KEY_H, KEY_J, KEY_K, KEY_L,
		KEY_SEMICOLON, KEY_APOSTROPHE, KEY_GRAVE, KEY_LEFTSHIFT, KEY_BACKSLASH,
		KEY_Z, KEY_X, KEY_C, KEY_V, KEY_B, KEY_N, KEY_M,
		KEY_COMMA, KEY_DOT, KEY_SLASH, KEY_RIGHTSHIFT, KEY_KPASTERISK,
		KEY_LEFTALT, KEY_SPACE, KEY_CAPSLOCK,
		KEY_F1, KEY_F2, KEY_F3, KEY_F4, KEY_F5, KEY_F6, KEY_F7, KEY_F8, KEY_F9, KEY_F10, KEY_F11, KEY_F12,
		KEY_UP, KEY_DOWN, KEY_LEFT, KEY_RIGHT,
		KEY_LEFTMETA, KEY_RIGHTMETA, KEY_RIGHTCTRL, KEY_RIGHTALT,
		KEY_DELETE, KEY_HOME, KEY_END, KEY_PAGEUP, KEY_PAGEDOWN, KEY_INSERT,
		BTN_LEFT, BTN_RIGHT, BTN_MIDDLE,
	}

	for _, key := range keys {
		if err := ioctl(fd, UI_SET_KEYBIT, uintptr(key)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("failed to enable key %d: %v", key, err)
		}
	}

	// Enable mouse movement
	rels := []uint16{REL_X, REL_Y, REL_WHEEL}
	for _, rel := range rels {
		if err := ioctl(fd, UI_SET_RELBIT, uintptr(rel)); err != nil {
			syscall.Close(fd)
			return nil, fmt.Errorf("failed to enable rel %d: %v", rel, err)
		}
	}

	// Setup device
	setup := UInputSetup{
		ID: InputID{
			Bustype: 0x03, // USB
			Vendor:  0x1234,
			Product: 0x5678,
			Version: 1,
		},
	}
	copy(setup.Name[:], "Virtual Input Device")

	if err := ioctlSetup(fd, UI_DEV_SETUP, &setup); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to setup device: %v", err)
	}

	// Create device
	if err := ioctl(fd, UI_DEV_CREATE, 0); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to create device: %v", err)
	}

	// Create key mapping
	keyMap := map[string]uint16{
		// Letters
		"a": KEY_A, "b": KEY_B, "c": KEY_C, "d": KEY_D, "e": KEY_E,
		"f": KEY_F, "g": KEY_G, "h": KEY_H, "i": KEY_I, "j": KEY_J,
		"k": KEY_K, "l": KEY_L, "m": KEY_M, "n": KEY_N, "o": KEY_O,
		"p": KEY_P, "q": KEY_Q, "r": KEY_R, "s": KEY_S, "t": KEY_T,
		"u": KEY_U, "v": KEY_V, "w": KEY_W, "x": KEY_X, "y": KEY_Y, "z": KEY_Z,
		
		// Numbers
		"1": KEY_1, "2": KEY_2, "3": KEY_3, "4": KEY_4, "5": KEY_5,
		"6": KEY_6, "7": KEY_7, "8": KEY_8, "9": KEY_9, "0": KEY_0,
		
		// Special keys
		"space": KEY_SPACE, "enter": KEY_ENTER, "tab": KEY_TAB,
		"backspace": KEY_BACKSPACE, "delete": KEY_DELETE, "esc": KEY_ESC,
		"escape": KEY_ESC,
		
		// Modifiers
		"ctrl": KEY_LEFTCTRL, "leftctrl": KEY_LEFTCTRL, "rightctrl": KEY_RIGHTCTRL,
		"shift": KEY_LEFTSHIFT, "leftshift": KEY_LEFTSHIFT, "rightshift": KEY_RIGHTSHIFT,
		"alt": KEY_LEFTALT, "leftalt": KEY_LEFTALT, "rightalt": KEY_RIGHTALT,
		"super": KEY_LEFTMETA, "meta": KEY_LEFTMETA, "leftmeta": KEY_LEFTMETA, "rightmeta": KEY_RIGHTMETA,
		"win": KEY_LEFTMETA, "windows": KEY_LEFTMETA, "cmd": KEY_LEFTMETA,
		
		// Arrow keys
		"up": KEY_UP, "down": KEY_DOWN, "left": KEY_LEFT, "right": KEY_RIGHT,
		
		// Function keys
		"f1": KEY_F1, "f2": KEY_F2, "f3": KEY_F3, "f4": KEY_F4,
		"f5": KEY_F5, "f6": KEY_F6, "f7": KEY_F7, "f8": KEY_F8,
		"f9": KEY_F9, "f10": KEY_F10, "f11": KEY_F11, "f12": KEY_F12,
		
		// Other keys
		"home": KEY_HOME, "end": KEY_END, "pageup": KEY_PAGEUP, "pagedown": KEY_PAGEDOWN,
		"insert": KEY_INSERT, "capslock": KEY_CAPSLOCK,
		"semicolon": KEY_SEMICOLON, "apostrophe": KEY_APOSTROPHE, "grave": KEY_GRAVE,
		"minus": KEY_MINUS, "equal": KEY_EQUAL, "leftbrace": KEY_LEFTBRACE,
		"rightbrace": KEY_RIGHTBRACE, "backslash": KEY_BACKSLASH,
		"comma": KEY_COMMA, "dot": KEY_DOT, "slash": KEY_SLASH,
		
		// Mouse buttons
		"leftclick": BTN_LEFT, "rightclick": BTN_RIGHT, "middleclick": BTN_MIDDLE,
	}

	return &VirtualInputDevice{fd: fd, keyMap: keyMap}, nil
}

func (vid *VirtualInputDevice) sendEvent(eventType, code uint16, value int32) error {
	event := InputEvent{
		Time:  syscall.NsecToTimeval(time.Now().UnixNano()),
		Type:  eventType,
		Code:  code,
		Value: value,
	}

	_, err := syscall.Write(vid.fd, (*(*[unsafe.Sizeof(event)]byte)(unsafe.Pointer(&event)))[:])
	return err
}

func (vid *VirtualInputDevice) HoldKeys(keys []string) error {
	for _, keyName := range keys {
		if key, exists := vid.keyMap[strings.ToLower(keyName)]; exists {
			if err := vid.sendEvent(EV_KEY, key, 1); err != nil {
				return fmt.Errorf("failed to hold key %s: %v", keyName, err)
			}
		} else {
			return fmt.Errorf("unknown key: %s", keyName)
		}
	}
	return vid.sendEvent(EV_SYN, 0, 0)
}

func (vid *VirtualInputDevice) ReleaseKeys(keys []string) error {
	for _, keyName := range keys {
		if key, exists := vid.keyMap[strings.ToLower(keyName)]; exists {
			if err := vid.sendEvent(EV_KEY, key, 0); err != nil {
				return fmt.Errorf("failed to release key %s: %v", keyName, err)
			}
		} else {
			return fmt.Errorf("unknown key: %s", keyName)
		}
	}
	return vid.sendEvent(EV_SYN, 0, 0)
}

func (vid *VirtualInputDevice) PressKeys(keys []string) error {
	// Hold all keys
	if err := vid.HoldKeys(keys); err != nil {
		return err
	}
	
	// Small delay
	time.Sleep(50 * time.Millisecond)
	
	// Release all keys
	return vid.ReleaseKeys(keys)
}

func (vid *VirtualInputDevice) MoveMouse(x, y int32) error {
	if err := vid.sendEvent(EV_REL, REL_X, x); err != nil {
		return err
	}
	if err := vid.sendEvent(EV_REL, REL_Y, y); err != nil {
		return err
	}
	return vid.sendEvent(EV_SYN, 0, 0)
}

func (vid *VirtualInputDevice) ClickMouse(button string) error {
	var btn uint16
	switch strings.ToLower(button) {
	case "left", "":
		btn = BTN_LEFT
	case "right":
		btn = BTN_RIGHT
	case "middle":
		btn = BTN_MIDDLE
	default:
		return fmt.Errorf("unknown mouse button: %s", button)
	}

	if err := vid.sendEvent(EV_KEY, btn, 1); err != nil {
		return err
	}
	if err := vid.sendEvent(EV_KEY, btn, 0); err != nil {
		return err
	}
	return vid.sendEvent(EV_SYN, 0, 0)
}

func (vid *VirtualInputDevice) ScrollMouse(direction int32) error {
	if err := vid.sendEvent(EV_REL, REL_WHEEL, direction); err != nil {
		return err
	}
	return vid.sendEvent(EV_SYN, 0, 0)
}

func (vid *VirtualInputDevice) Close() error {
	ioctl(vid.fd, UI_DEV_DESTROY, 0)
	return syscall.Close(vid.fd)
}

// Helper functions for ioctl calls
func ioctl(fd int, cmd uintptr, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), cmd, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlSetup(fd int, cmd uintptr, setup *UInputSetup) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), cmd, uintptr(unsafe.Pointer(setup)))
	if errno != 0 {
		return errno
	}
	return nil
}

// Global virtual input device
var device *VirtualInputDevice

// HTTP handlers
func holdHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, Response{false, "Invalid JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if len(req.Keys) == 0 {
		respondJSON(w, Response{false, "No keys specified"}, http.StatusBadRequest)
		return
	}

	if err := device.HoldKeys(req.Keys); err != nil {
		respondJSON(w, Response{false, "Failed to hold keys: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, Response{true, fmt.Sprintf("Holding keys: %v", req.Keys)}, http.StatusOK)
}

func releaseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, Response{false, "Invalid JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if len(req.Keys) == 0 {
		respondJSON(w, Response{false, "No keys specified"}, http.StatusBadRequest)
		return
	}

	if err := device.ReleaseKeys(req.Keys); err != nil {
		respondJSON(w, Response{false, "Failed to release keys: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, Response{true, fmt.Sprintf("Released keys: %v", req.Keys)}, http.StatusOK)
}

func pressHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, Response{false, "Invalid JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if len(req.Keys) == 0 {
		respondJSON(w, Response{false, "No keys specified"}, http.StatusBadRequest)
		return
	}

	if err := device.PressKeys(req.Keys); err != nil {
		respondJSON(w, Response{false, "Failed to press keys: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, Response{true, fmt.Sprintf("Pressed keys: %v", req.Keys)}, http.StatusOK)
}

func mouseMoveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MouseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, Response{false, "Invalid JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if err := device.MoveMouse(req.X, req.Y); err != nil {
		respondJSON(w, Response{false, "Failed to move mouse: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, Response{true, fmt.Sprintf("Moved mouse by (%d, %d)", req.X, req.Y)}, http.StatusOK)
}

func mouseClickHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MouseClickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, Response{false, "Invalid JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if req.Button == "" {
		req.Button = "left"
	}

	if err := device.ClickMouse(req.Button); err != nil {
		respondJSON(w, Response{false, "Failed to click mouse: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, Response{true, fmt.Sprintf("Clicked %s mouse button", req.Button)}, http.StatusOK)
}

func mouseScrollHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MouseScrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, Response{false, "Invalid JSON: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if err := device.ScrollMouse(req.Direction); err != nil {
		respondJSON(w, Response{false, "Failed to scroll mouse: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	direction := "up"
	if req.Direction < 0 {
		direction = "down"
	}

	respondJSON(w, Response{true, fmt.Sprintf("Scrolled %s", direction)}, http.StatusOK)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	keys := make([]string, 0, len(device.keyMap))
	for key := range device.keyMap {
		keys = append(keys, key)
	}

	status := map[string]interface{}{
		"status": "running",
		"available_keys": keys,
		"endpoints": map[string]string{
			"hold":         "POST /hold - Hold keys down",
			"release":      "POST /release - Release held keys",
			"press":        "POST /press - Press and release keys",
			"mouse/move":   "POST /mouse/move - Move mouse",
			"mouse/click":  "POST /mouse/click - Click mouse button",
			"mouse/scroll": "POST /mouse/scroll - Scroll mouse wheel",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func respondJSON(w http.ResponseWriter, response Response, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Create virtual input device
	var err error
	device, err = NewVirtualInputDevice()
	if err != nil {
		log.Fatalf("Failed to create virtual input device: %v", err)
	}
	defer device.Close()

	// Setup HTTP routes
	http.HandleFunc("/hold", holdHandler)
	http.HandleFunc("/release", releaseHandler)
	http.HandleFunc("/press", pressHandler)
	http.HandleFunc("/mouse/move", mouseMoveHandler)
	http.HandleFunc("/mouse/click", mouseClickHandler)
	http.HandleFunc("/mouse/scroll", mouseScrollHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/", statusHandler) // Root shows status

	// Setup signal handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down...")
		device.Close()
		os.Exit(0)
	}()

	// Start server on all interfaces
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Printf("Virtual Input HTTP Server starting on 0.0.0.0:%s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /hold - Hold keys down")
	log.Printf("  POST /release - Release held keys")
	log.Printf("  POST /press - Press and release keys")
	log.Printf("  POST /mouse/move - Move mouse")
	log.Printf("  POST /mouse/click - Click mouse button")
	log.Printf("  POST /mouse/scroll - Scroll mouse wheel")
	log.Printf("  GET  /status - Show status and available keys")

	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
