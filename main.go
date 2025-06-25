package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var OpCodes = []string{
	"NOP",
	"LDA",
	"ADD",
	"SUB",
	"STA",
	"LDI",
	"JMP",
	"JC ",
	"JZ ",
	"---",
	"---",
	"---",
	"---",
	"---",
	"OUT",
	"HLT",
}

var Program = []byte{
	0x51, // 0 LDI 1
	0xe0, // 1 OUT
	0x2f, // 2 ADD 15
	0x7d, // 3 JC 13
	0xe0, // 4 OUT
	0x4e, // 5 STA 14
	0x1f, // 6 LDA 15
	0x2e, // 7 ADD 14
	0x7d, // 8 JC 13
	0xe0, // 9 OUT
	0x4f, // a STA 15
	0x1e, // b LDA 14
	0x62, // c JMP 2
	0xf0, // d HLT 0
	0x01, // e 1
	0x01, // f 1
}

// Program := []byte{
// 	0x1e, // 0x0	LDA 14  0x1e
// 	0x2f, // 0x2	ADD 15  0x2f
// 	0xe0, // 0x4	OUT		0xe0
// 	0xf0, // 0x6	HLT		0xf0
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x00,
// 	0x1c, // 0xe  28		0x1c
// 	0x0e, // 0xf  14		0x0e
// }

// Control Word Segments
const HLT uint16 = 0x8000 // Halt
const MI uint16 = 0x4000  // MAR In
const RI uint16 = 0x2000  // RAM In
const RO uint16 = 0x1000  // RAM Out
const IO uint16 = 0x0800  // IR Out
const II uint16 = 0x0400  // IR In
const AI uint16 = 0x0200  // A Register In
const AO uint16 = 0x0100  // A Register Out
const EO uint16 = 0x0080  // ALU Out
const SU uint16 = 0x0040  // Subtract Mode
const BI uint16 = 0x0020  // B Register In
const OI uint16 = 0x0010  // Output In
const CE uint16 = 0x0008  // Counter Enable
const CO uint16 = 0x0004  // Counter Out
const J uint16 = 0x0002   // Jump
const FI uint16 = 0x0001  // Flags In

// registers
var ProgramCounter byte = 0
var ARegister byte = 0
var BRegister byte = 0
var ERegister byte = 0
var OverflowBit bool = false
var ZeroBit = true
var InstructionRegister byte = 0
var MemoryRegister byte = 0
var Output byte = 0
var CarryFlag = false
var ZeroFlag = false

var CurrentOpCode string = "???"
var CurrentOperand byte = 0
var CurrRomAddress uint16 = 0
var ControlWord uint16 = 0
var ClockPulse int = 0
var BusData byte = 0
var Memory [16]byte
var Rom [1024]byte
var run = false

var b2i = map[bool]int{false: 0, true: 1}

func main() {
	args := os.Args[1:]
	for _, arg := range args {
		switch arg {
		case "--run":
			run = true
		case "-r":
			run = true
		}
	}

	Rom = *BuildRom()

	copy(Memory[0:], Program)

	fmt.Println("************************* Starting Processor Execution *************************")
	fmt.Print("\n\n\n\n\n\n\n\n\n\n")

	for {
		for ClockPulse = range 5 {
			CalculateRomAddress(ClockPulse)
			ControlWord = RomLookup(CurrRomAddress)
			switch ClockPulse {
			case 0: // fetch 1
				BusData = ProgramCounter // CO
				MemoryRegister = BusData // MI
				CurrentOpCode = "???"
				CurrentOperand = 0
			case 1: // fetch 2
				BusData = Memory[MemoryRegister] // RO
				InstructionRegister = BusData    // II
				ProgramCounter++                 // CE
				CurrentOpCode = OpCodes[InstructionRegister>>4]
				CurrentOperand = InstructionRegister & 0x0f
			default: // cases 2, 3, 4
				BusData = 0
				ExecuteControlWord()
			}
			PrintStatus()
			if run {
				fmt.Println()
				time.Sleep(time.Millisecond * 50)
			} else {
				fmt.Scanln()
			}
		}
	}
}

func ExecuteControlWord() {
	if ControlWord&HLT != 0 {
		PrintStatus()
		fmt.Println("*********************** Received a HALT signal *********************************")
		os.Exit(0)
	}
	// Put data on bus (first match wins since it's undefined to have more than one thing on the bus)
	switch {
	case (ControlWord&RO != 0):
		BusData = Memory[MemoryRegister]
	case (ControlWord&IO != 0):
		BusData = InstructionRegister & 0x0f // only the lower 4 bits
	case (ControlWord&AO != 0):
		BusData = ARegister
	case (ControlWord&EO != 0):
		BusData = ERegister
	case (ControlWord&CO != 0):
		BusData = ProgramCounter
	}

	if ControlWord&CE != 0 {
		ProgramCounter++
	}

	// Load data from the bus (all listeners load because several can read at once)
	// Load Flags first before they get reset
	if ControlWord&FI != 0 {
		ZeroFlag = ZeroBit
		CarryFlag = OverflowBit
	}
	// A and B first
	if ControlWord&AI != 0 {
		ARegister = BusData
	}
	if ControlWord&BI != 0 {
		BRegister = BusData
	}
	// Update ERegister and Overflow and Zero bits
	if ControlWord&SU != 0 { // Subtracting
		ERegister = ARegister - BRegister
		OverflowBit = ((^ARegister&BRegister)|(^(ARegister^BRegister)&ERegister))>>7 != 0
	} else { // Adding (default)
		var sum uint16 = uint16(ARegister) + uint16(BRegister)
		ERegister = byte(sum)
		OverflowBit = sum>>8 != 0
	}
	ZeroBit = ERegister == 0

	// The rest of the reads
	if ControlWord&MI != 0 {
		MemoryRegister = BusData
	}
	if ControlWord&RI != 0 {
		Memory[MemoryRegister&0x0F] = BusData
	}
	if ControlWord&II != 0 {
		InstructionRegister = BusData
	}
	if ControlWord&OI != 0 {
		Output = BusData
	}
	if ControlWord&J != 0 {
		ProgramCounter = BusData
	}
}

// Microcode Address Calculation
//
//	Z C OPCD CLK
//	Where:
//	  Z = Zero Flag
//	  C = Carry Flag
//	  OPCD = 4-bit OpCode
//	  CLK = 3-bit microcode step (0b000 - 0b100)
func CalculateRomAddress(ClockPulse int) {
	var Z uint16 = 0
	if ZeroFlag {
		Z = 0x100
	}
	var C uint16 = 0
	if CarryFlag {
		C = 0x80
	}
	CurrRomAddress = Z | C | uint16(InstructionRegister>>4)<<3 | uint16(ClockPulse)
}

func RomLookup(RomAddress uint16) uint16 {
	return uint16(Rom[RomAddress])<<8 | uint16(Rom[RomAddress+0x200])
}

func PrintStatus() {
	fmt.Print("\033[9A")
	fmt.Printf("Bus Contents: %08b   Memory Addr: 0x%02x      PrgCnt: 0x%0x  Tick: 0x%0x\n", BusData, MemoryRegister, ProgramCounter, ClockPulse)
	fmt.Printf("Mem Contents: %08b   A Register: %08b   CF: %d  ZF: %d\n", Memory[MemoryRegister], ARegister, b2i[CarryFlag], b2i[ZeroFlag])
	fmt.Printf("Inst Reg:   %04b %04b    B Register: %08b   Output: ** %3d **\n", InstructionRegister>>4, InstructionRegister&0x0f, BRegister, Output)
	fmt.Printf("Instruction: %s  %3d    E Register: %08b\n", CurrentOpCode, CurrentOperand, ERegister)
	fmt.Printf("Control Word: %s\n", ConstructControlWord(ControlWord))
	fmt.Println()
	fmt.Printf("RAM:")
	for i := range 0x10 {
		fmt.Printf(" %02x", Memory[i])
	}
	fmt.Println()
	fmt.Println()
}

func BuildRom() *[1024]byte {
	fmt.Println()
	fmt.Println("****************************** Starting ROM Build ******************************")

	UCODE_TEMPLATE := [16][8]uint16{
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 0000 - NOP
		{MI | CO, RO | II | CE, IO | MI, RO | AI, 0, 0, 0, 0},                 // 0001 - LDA
		{MI | CO, RO | II | CE, IO | MI, RO | BI, EO | AI | FI, 0, 0, 0},      // 0010 - ADD
		{MI | CO, RO | II | CE, IO | MI, RO | BI, EO | AI | SU | FI, 0, 0, 0}, // 0011 - SUB
		{MI | CO, RO | II | CE, IO | MI, AO | RI, 0, 0, 0, 0},                 // 0100 - STA
		{MI | CO, RO | II | CE, IO | AI, 0, 0, 0, 0, 0},                       // 0101 - LDI
		{MI | CO, RO | II | CE, IO | J, 0, 0, 0, 0, 0},                        // 0110 - JMP
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 0111 - JC
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 1000 - JZ
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 1001
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 1010
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 1011
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 1100
		{MI | CO, RO | II | CE, 0, 0, 0, 0, 0, 0},                             // 1101
		{MI | CO, RO | II | CE, AO | OI, 0, 0, 0, 0, 0},                       // 1110 - OUT
		{MI | CO, RO | II | CE, HLT, 0, 0, 0, 0, 0},                           // 1111 - HLT
	}

	var ucode [4][16][8]uint16
	for i := range 4 {
		ucode[i] = UCODE_TEMPLATE
	}

	// ZF=0, CF=1
	ucode[1][0x7][2] = IO | J
	// ZF=1, CF=0
	ucode[2][0x8][2] = IO | J
	// ZF=1, CF=1
	ucode[3][0x7][2] = IO | J
	ucode[3][0x8][2] = IO | J

	var RomBytes [1024]byte
	var RomAddress int = 0
	for i := range 2 { // write the first byte of each word to the first half and the second byte to the second half
		for j := range 4 { // one set for each of the 4 combinations of ZF and CF
			for k := range 16 { // total of 16 OpCodes
				for l := range 8 { // 8 possible uCodes per step so 8 slots for Control Words
					if i == 0 {
						RomBytes[RomAddress] = byte(ucode[j][k][l] >> 8) // first byte of the word
					} else {
						RomBytes[RomAddress] = byte(ucode[j][k][l]) // second byte of the word
					}
					RomAddress++
				}
			}
		}
	}

	// write the ROM to a file
	err := os.WriteFile("rom.bin", RomBytes[:], 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing ROM file out: %v\n", err)
	}

	fmt.Println()
	fmt.Println("****************************** ROM Build Complete ******************************")
	fmt.Println()
	return &RomBytes
}

func ConstructControlWord(ControlWord uint16) string {
	Labels := []string{"HL", "MI", "RI", "RO", "IO", "II", "AI", "AO", "EO", "SU", "BI", "OI", "CE", "CO", "J ", "FI"}
	Segments := []uint16{HLT, MI, RI, RO, IO, II, AI, AO, EO, SU, BI, OI, CE, CO, J, FI}
	var Builder strings.Builder
	for i := range 16 {
		if ControlWord&Segments[i] > 0 {
			Builder.WriteString(fmt.Sprintf("%s ", Labels[i]))
		} else {
			Builder.WriteString("__ ")
		}
	}
	return Builder.String()
}

// func printStatusBig() {
// 	fmt.Printf("\n")
// 	fmt.Printf("\n")
// 	fmt.Printf("                   8-bit Bus.\n")
// 	fmt.Printf("                   ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉\n")
// 	fmt.Printf("\n")
// 	fmt.Printf("   Memory Address                      Counter\n")
// 	fmt.Printf("   ◉ ◉ ◉ ◉                             ◉ ◉ ◉ ◉\n")
// 	fmt.Printf("\n")
// 	fmt.Printf("   Memory Contents                     A Register              Flags\n")
// 	fmt.Printf("   ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉                     ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉         ◉ ◉\n")
// 	fmt.Printf("                                                               C Z\n")
// 	fmt.Printf("                                       Σ Register              F F\n")
// 	fmt.Printf("                                       ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉\n")
// 	fmt.Printf("   Instruction Register\n")
// 	fmt.Printf("   ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉                     B Register              Output\n")
// 	fmt.Printf("                                       ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉          _    _    _    _\n")
// 	fmt.Printf("                                                               |_|  |_|  |_|  |_|\n")
// 	fmt.Printf("          T T T T T                                            |_|  |_|  |_|  |_|\n")
// 	fmt.Printf("   Step   0 1 2 3 4\n")
// 	fmt.Printf("   ◉ ◉ ◉  ◉ ◉ ◉ ◉ ◉                    Control Word\n")
// 	fmt.Printf("                                       ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉ ◉\n")
// 	fmt.Printf("                                       H M R R I I A A E S B O C C J F\n")
// 	fmt.Printf("                                       L I I O O I I O O U I I E O   I\n")
// 	fmt.Printf("                                       T\n")
// 	fmt.Printf("\n")
// 	fmt.Printf("\n")
// }
