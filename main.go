package main

import (
	"fmt"
	"image"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xgraphics"
	"github.com/BurntSushi/xgbutil/xwindow"
)

var DisplayWindow *xwindow.Window
var WindowImage *xgraphics.Image
var X *xgbutil.XUtil

type Instruction uint32
type XY [2]int

// Static address list (relative to a center pixel)
const (
	PROGRAM_COUNTER uint32 = iota // 24 bit program counter
	STACK_POINTER                 // 24 bit stack pointer
	REGISTER_A                    // Register A
	REGISTER_B                    // Register B
	PROGRAM                       // Start of program
)

// Instructions (4 bits instruction, 20 bits data):
const (
	NOP   Instruction = iota
	JUMP              // Relative jump (Add data (signed) to PC)
	COND              // Conditional jump (If A is non-zero, perform relative jump)
	SETA              // Set A (Set A to data (unsigned))
	ADDI              // Add immediate (Add data (signed) to A)
	PUSH              // Push stack (Push A to stack)
	POP               // Pop stack (Pop from stack and store in A)
	ADDS              // Add stack (Pop from stack and add to A)
	SWAP              // Swap A and B
	RAND              // Add random value to A (A = rand % data, unless data == 0)
	SHIFT             // Shift A (Shift contents of A by data (signed))
	LOCAL             // Set A to the current program's x,y coordinates
	RPUSH             // Push remote (Push A to remote stack of x,y = B)
	RSET              // Write remote (Store A at x,y = B, r = data)
	RGET              // Read remote (Read A from x,y = B, r = data)
	FORK              // Fork (call go Exec(B))
)

func (i Instruction) String() string {
	return []string{"NOP", "JUMP", "COND", "SETA", "ADDI", "PUSH", "POP", "ADDS", "SWAP", "RAND", "SHIFT", "LOCAL", "RPUSH", "RSET", "RGET", "FORK"}[i]
}

func WriteProgram(x, y uint32) {
	r := PROGRAM
	// Padding for static memory usage
	WriteInstruction(x, y, &r, JUMP, 3)
	mem1 := r
	WriteInstruction(x, y, &r, NOP, 0)
	mem2 := r
	WriteInstruction(x, y, &r, NOP, 0)

	{ // Store end of program in the static slot 1
		WriteInstruction(x, y, &r, LOCAL, 0)
		WriteInstruction(x, y, &r, SWAP, 0)
		WriteInstruction(x, y, &r, RGET, int32(STACK_POINTER))
		WriteInstruction(x, y, &r, RSET, int32(mem1))
	}
	start := r
	{ // for i := 1024; i != 0; i-- {
		WriteInstruction(x, y, &r, SETA, 1024)
		loop := r
		{ // Push random blue to stack
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, RAND, 255)
			WriteInstruction(x, y, &r, PUSH, 0)
			WriteInstruction(x, y, &r, SWAP, 0)
		}
		WriteInstruction(x, y, &r, ADDI, -1)
		WriteInstruction(x, y, &r, COND, int32(loop-r))
	}
	{ // for i := 1024; i != 0; i-- {
		WriteInstruction(x, y, &r, SETA, 1024)
		loop := r
		{ // Push random green to stack
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, RAND, 255)
			WriteInstruction(x, y, &r, SHIFT, 8)
			WriteInstruction(x, y, &r, PUSH, 0)
			WriteInstruction(x, y, &r, SWAP, 0)
		}
		WriteInstruction(x, y, &r, ADDI, -1)
		WriteInstruction(x, y, &r, COND, int32(loop-r))
	}
	{ // for i := 1024; i != 0; i-- {
		WriteInstruction(x, y, &r, SETA, 1024)
		loop := r
		{ // Push random red to stack
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, RAND, 255)
			WriteInstruction(x, y, &r, SHIFT, 16)
			WriteInstruction(x, y, &r, PUSH, 0)
			WriteInstruction(x, y, &r, SWAP, 0)
		}
		WriteInstruction(x, y, &r, ADDI, -1)
		WriteInstruction(x, y, &r, COND, int32(loop-r))
	}
	{ // Copy the current program
		{ // Store current stack pointer in static slot 2
			WriteInstruction(x, y, &r, LOCAL, 0)
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, RGET, int32(STACK_POINTER))
			WriteInstruction(x, y, &r, RSET, int32(mem2))
		}
		{ // Set stack pointer to end of program
			WriteInstruction(x, y, &r, RGET, int32(mem1))
			WriteInstruction(x, y, &r, RSET, int32(STACK_POINTER))
		}
		{ // Select random coordinates for new program and set stack pointer to end of remote program
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, SETA, 0)
			WriteInstruction(x, y, &r, RAND, 1024)
			WriteInstruction(x, y, &r, SHIFT, 12)
			WriteInstruction(x, y, &r, RAND, 1024)
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, ADDI, -1)
			WriteInstruction(x, y, &r, RSET, int32(STACK_POINTER))
		}
		{ // Pop from local stack, push to local stack in reverse order
			WriteInstruction(x, y, &r, ADDI, -1)
			loop := r
			WriteInstruction(x, y, &r, ADDI, 1)
			WriteInstruction(x, y, &r, POP, 0)
			WriteInstruction(x, y, &r, RPUSH, 0)
			WriteInstruction(x, y, &r, RGET, int32(STACK_POINTER))
			WriteInstruction(x, y, &r, ADDI, -2)
			WriteInstruction(x, y, &r, RSET, int32(STACK_POINTER))
			WriteInstruction(x, y, &r, ADDI, -1)
			WriteInstruction(x, y, &r, COND, int32(loop-r))
		}
		{ // Fork remote program
			WriteInstruction(x, y, &r, RGET, int32(mem1)) // Restore remote stack pointer
			WriteInstruction(x, y, &r, RSET, int32(STACK_POINTER))
			WriteInstruction(x, y, &r, SETA, 0)
			WriteInstruction(x, y, &r, RSET, int32(PROGRAM_COUNTER))
			WriteInstruction(x, y, &r, FORK, 0)
		}
		{ // Restore local stack
			WriteInstruction(x, y, &r, LOCAL, 0)
			WriteInstruction(x, y, &r, SWAP, 0)
			WriteInstruction(x, y, &r, RGET, int32(mem2))
			WriteInstruction(x, y, &r, RSET, int32(STACK_POINTER))
		}
	}
	WriteInstruction(x, y, &r, JUMP, int32(start-r))

	data, _ := Coords(x, y, PROGRAM_COUNTER)
	Write24Bit(data, 0) // Initial program counter
	data, _ = Coords(x, y, STACK_POINTER)
	Write24Bit(data, r) // Set stack pointer to end of program

	// fmt.Println("Program length:", r)
}

func main() {
	var err error
	X, err = xgbutil.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	// Create a window
	DisplayWindow, err = xwindow.Generate(X)
	if err != nil {
		log.Fatalf("Could not generate a new window X id: %s", err)
	}
	DisplayWindow.Create(X.RootWin(), 0, 0, 1024, 1024, xproto.CwBackPixel, 0)
	DisplayWindow.Map()

	WindowImage = xgraphics.New(X, image.Rect(0, 0, 1024, 1024))

	err = WindowImage.XSurfaceSet(DisplayWindow.Id)
	if err != nil {
		log.Printf("Error while setting window surface to image %d: %s\n", DisplayWindow, err)
	}

	// Start a program in the center of the screen if no programs are running
	go func() {
		for {
			count := atomic.SwapInt64(&forkCount, 0)
			if count < 1 {
				fmt.Println("No forks in the last 10 seconds, spawning new program")
				WriteProgram(512, 512)
				go Exec(512, 512)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	drawTimer = time.Tick(16 * time.Millisecond)
	printTimer = time.Tick(1 * time.Second)

	xevent.Main(X)
}

var execCount int64 = 0
var forkCount int64 = 0
var fpsCount int64 = 0
var drawTimer <-chan time.Time
var printTimer <-chan time.Time
var execs [1024][1024]uint32

func Exec(x, y uint32) {
	if int(x) >= WindowImage.Rect.Max.X || int(y) >= WindowImage.Rect.Max.Y {
		return
	}
	count := atomic.AddInt64(&execCount, 1)
	atomic.AddInt64(&forkCount, 1)
	defer atomic.AddInt64(&execCount, -1)
	if count > 100 {
		return
	}
	old := atomic.SwapUint32(&execs[x][y], 1)
	if old != 0 {
		return
	}
	defer atomic.StoreUint32(&execs[x][y], 0)
	// fmt.Println("Starting program at", x, y)
	for {
		// Read Program Counter
		mem, ok := Coords(x, y, PROGRAM_COUNTER)
		if !ok {
			// fmt.Println("Program", x, y, "invalid")
			return
		}
		PC := Read24Bit(mem)

		// Read Stack Pointer
		mem, ok = Coords(x, y, STACK_POINTER)
		if !ok {
			// fmt.Println("Program", x, y, "has invalid stack pointer")
			return
		}
		S := Read24Bit(mem)

		// Read Register A
		mem, ok = Coords(x, y, REGISTER_A)
		if !ok {
			// fmt.Println("Program", x, y, "has invalid register A")
			return
		}
		A := Read24Bit(mem)

		// Read Register B
		mem, ok = Coords(x, y, REGISTER_B)
		if !ok {
			// fmt.Println("Program", x, y, "has invalid register B")
			return
		}
		B := Read24Bit(mem)

		// Read instruction
		mem, ok = Coords(x, y, PROGRAM+PC)
		if !ok {
			// fmt.Println("Program", x, y, "has invalid PC:", PC)
			return
		}
		instrData := Read24Bit(mem)
		instr := Instruction((instrData & 0xF00000) >> 20)
		data := instrData & 0xFFFFF

		// Evaluate instruction
		// fmt.Printf("(%d, %d): PC(%4d) S(%4d) A(%10d) B(%10d) Instruction(%s)\tData(%d)\n", x, y, PC, S, A, B, instr, int32(Signed20Bit(data)))
		switch instr {
		case NOP: // NOP
		case JUMP: // Relative jump
			PC += Signed20Bit(data) - 1
		case COND: // Conditional jump
			if A != 0 {
				PC += Signed20Bit(data) - 1
			}
		case SETA: // Set register A
			mem, _ = Coords(x, y, REGISTER_A)
			Write24Bit(mem, data)
		case ADDI: // Add immediate to A
			mem, _ = Coords(x, y, REGISTER_A)
			Write24Bit(mem, A+Signed20Bit(data))
		case PUSH: // Push A to stack
			mem, ok = Coords(x, y, S)
			if !ok {
				// fmt.Println("Program", x, y, "has stack overflow:", S)
				return
			}
			Write24Bit(mem, A)
			mem, _ = Coords(x, y, STACK_POINTER)
			Write24Bit(mem, S+1)
		case POP: // Pop stack into A
			mem, ok = Coords(x, y, S-1)
			if !ok {
				// fmt.Println("Program", x, y, "has stack overflow:", S-1)
				return
			}
			r := Read24Bit(mem)
			mem, _ = Coords(x, y, STACK_POINTER)
			Write24Bit(mem, S-1)
			mem, _ = Coords(x, y, REGISTER_A)
			Write24Bit(mem, r)
		case ADDS: // Pop from stack and add to A
			mem, ok = Coords(x, y, S-1)
			if !ok {
				// fmt.Println("Program", x, y, "has stack overflow:", S-1)
				return
			}
			r := Read24Bit(mem)
			mem, _ = Coords(x, y, STACK_POINTER)
			Write24Bit(mem, S-1)
			mem, _ = Coords(x, y, REGISTER_A)
			Write24Bit(mem, A+r)
		case SWAP: // Swap registers A and B
			mem, _ = Coords(x, y, REGISTER_B)
			Write24Bit(mem, A)
			mem, _ = Coords(x, y, REGISTER_A)
			Write24Bit(mem, B)
		case RAND: // Add random bits to A
			mem, _ = Coords(x, y, REGISTER_A)
			r := rand.Uint32()
			if data != 0 {
				r = r % data
			}
			Write24Bit(mem, A+r)
		case SHIFT: // Shift A by data (signed)
			mem, _ = Coords(x, y, REGISTER_A)
			Write24Bit(mem, A<<Signed20Bit(data))
		case LOCAL: // Set A to x,y
			mem, _ = Coords(x, y, REGISTER_A)
			Write12Bit(mem, x, y)
		case RPUSH: // Push to remote stack
			mem, _ = Coords(x, y, REGISTER_B)
			x2, y2 := Read12Bit(mem)
			mem, ok = Coords(x2, y2, STACK_POINTER)
			if !ok {
				// fmt.Println("Program", x2, y2, "has invalid stack pointer")
			} else {
				S2 := Read24Bit(mem)
				mem, ok = Coords(x2, y2, S2)
				if !ok {
					// fmt.Println("Program", x2, y2, "has remote stack overflow:", S2)
				} else {
					Write24Bit(mem, A)
				}
				mem, _ = Coords(x2, y2, STACK_POINTER)
				Write24Bit(mem, S2+1)
			}
		case RSET: // Write A into remote address
			mem, _ = Coords(x, y, REGISTER_B)
			x2, y2 := Read12Bit(mem)
			mem, ok = Coords(x2, y2, data)
			if !ok {
				// fmt.Println("Program", x, y, "has invalid remote write:", x2, y2, data)
			} else {
				Write24Bit(mem, A)
			}
		case RGET: // Read remote address into A
			mem, _ = Coords(x, y, REGISTER_B)
			x2, y2 := Read12Bit(mem)
			mem, ok = Coords(x2, y2, data)
			if !ok {
				// fmt.Println("Program", x, y, "has invalid remote read:", x2, y2, data)
			} else {
				r := Read24Bit(mem)
				mem, _ = Coords(x, y, REGISTER_A)
				Write24Bit(mem, r)
			}
		case FORK: // Call go Exec(B)
			mem, _ = Coords(x, y, REGISTER_B)
			x2, y2 := Read12Bit(mem)
			go Exec(x2, y2)
		default:
			fmt.Println("Program", x, y, "running unknown instruction:", instr)
			return
		}

		// Advance program counter
		mem, _ = Coords(x, y, PROGRAM_COUNTER)
		Write24Bit(mem, PC+1)

		select {
		case <-drawTimer:
			atomic.AddInt64(&fpsCount, 1)
			DrawImage(WindowImage)
		case <-printTimer:
			execs := atomic.LoadInt64(&execCount)
			fps := atomic.SwapInt64(&fpsCount, 0)
			fmt.Println("FPS:", fps, "Current executions:", execs)
		default:
			time.Sleep(100 * time.Nanosecond)
		}
	}
}

// Write an instruction to x,y,r
func WriteInstruction(x, y uint32, r *uint32, instr Instruction, data int32) {
	mem, _ := Coords(x, y, *r)
	instrData := (uint32(instr&0xF) << 20) | uint32(data&0xFFFFF)
	Write24Bit(mem, instrData)
	*r++
}

// Read a 24 bit pixel as 2x 12 bit uints
func Read12Bit(in []uint8) (outx, outy uint32) {
	outx = uint32(in[0]) | (uint32(in[1]&0xF) << 8)
	outy = (uint32(in[1]&0xF0) >> 4) | (uint32(in[2]) << 4)
	return outx, outy
}

// Write 2x 12 bit uints as a 24 bit pixel
func Write12Bit(out []uint8, inx, iny uint32) {
	out[0] = uint8(inx & 0xFF)
	out[1] = uint8(((inx & 0xF00) >> 8) | ((iny & 0xF) << 4))
	out[2] = uint8(((iny & 0xFF0) >> 4))
}

// Read a 24 bit pixel as a 24 bit uint
func Read24Bit(in []uint8) (out uint32) {
	out = uint32(in[0]) | (uint32(in[1]) << 8) | (uint32(in[2]) << 16)
	return out
}

// Write a 24 bit uint as a 24 bit pixel
func Write24Bit(out []uint8, in uint32) {
	out[0] = uint8(in & 0xFF)
	out[1] = uint8(((in & 0xFF00) >> 8))
	out[2] = uint8(((in & 0xFF0000) >> 16))
}

// Convert a 2s complement 20-bit uint to a 32-bit uint
func Signed20Bit(in uint32) (out uint32) {
	out = in
	if (out & 0x80000) != 0 {
		// Data is negative
		out |= 0xFFF00000
	}
	return out
}

// Generate a roughly spiral shaped memory space.
// CoordOffset represents an XY offset relative to a center coordinate
// Larger indexes in CoordOffset will have a larger distance from center
var CoordOffset []XY

// Return the slice of pixel data for the psuedo-polar coordinates x,y,r
func Coords(x, y, r uint32) ([]uint8, bool) {
	if r >= uint32(len(CoordOffset)) {
		// log.Printf("Coord radius too large: %d\n", r)
		return []uint8{0, 0, 0}, false
	}
	dx := CoordOffset[r][0]
	dy := CoordOffset[r][1]
	if int(x)+dx < 0 || int(x)+dx >= WindowImage.Rect.Max.X || int(y)+dy < 0 || int(y)+dy >= WindowImage.Rect.Max.Y {
		return []uint8{0, 0, 0}, false
	}
	i := (int(x)+dx)*4 + (int(y)+dy)*WindowImage.Stride
	return WindowImage.Pix[i : i+3], true
}

var DirOffset = [8]XY{
	{0, -1},
	{-1, -1},
	{-1, 0},
	{-1, 1},
	{0, 1},
	{1, 1},
	{1, 0},
	{1, -1},
}

func init() {
	// Generate CoordOffset lookup table using a boundary fill algorithm
	var area [2048][2048]bool
	originx := 1024
	originy := 1024
	CoordOffset = []XY{{0, 0}, {0, 1}}
	area[originx][originy] = true
	area[originx][originy+1] = true
	dx := 0
	dy := 1
	for {
		var dir int
		if dx >= 0 {
			if dy > dx {
				dir = 0
			} else if dy >= 0 {
				dir = 1
			} else if dx > -dy {
				dir = 2
			} else {
				dir = 3
			}
		} else {
			if dy < dx {
				dir = 4
			} else if dy < 0 {
				dir = 5
			} else if -dx > dy {
				dir = 6
			} else {
				dir = 7
			}
		}
		for i := 0; i < 8; i++ {
			ddx := DirOffset[(dir+i)%8][0]
			ddy := DirOffset[(dir+i)%8][1]
			if dx+ddx < -originx || dx+ddx >= originx || dy+ddy < -originy || dy+ddy >= originy {
				return
			}
			if !area[originx+dx+ddx][originy+dy+ddy] {
				dx += ddx
				dy += ddy
				CoordOffset = append(CoordOffset, XY{dx, dy})
				area[originx+dx][originy+dy] = true
				break
			}
		}
	}
}

func DrawImage(img *xgraphics.Image) {
	img.XDraw()
	img.XPaint(DisplayWindow.Id)
}
