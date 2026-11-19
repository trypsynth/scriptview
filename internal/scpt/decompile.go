package scpt

import (
	"fmt"
	"strings"

	"github.com/trypsynth/scriptview/internal/mactext"
)

// nodeID identifies a node of the syntax tree, or a reference when it's
// negative. A file stores IDs in 16 bits, but the trees that the bytecode
// decompiler builds for large run-only scripts need more.
type nodeID int32

// decompState holds the full FASD parse tree needed for source reconstruction.
type decompState struct {
	nodes     map[nodeID]nodeRec
	nodeOrder []nodeID
	refName   map[nodeID]string
	intByRef  map[nodeID]int
	textByID  map[nodeID]string
	paramMap  map[nodeID][]nodeID
	// wraps holds 0x0e wrapper records (e.g. a literal's text payload).
	wraps    map[nodeID][]nodeID
	wrapType map[nodeID]byte
	// objects holds every decoded object by reference (strict parse only).
	objects map[nodeID]*object
	// valueCache memoizes decoded object values.
	valueCache map[nodeID]fasValue
	// canonical maps a lowercase identifier to its first spelling; Script
	// Editor spells interleaved handler name segments that way.
	canonical map[string]string
	appByID   map[nodeID]string
	// evCode holds the suite+event code of each command ref; refs without a
	// user-defined name are builtin commands, printed as `cmd arg`.
	evCode   map[nodeID]string
	realByID map[nodeID]float64
	descByID map[nodeID]descRec
	comments map[nodeID]string
	// osCode holds the raw four-char code behind each OSType ref, for
	// context-dependent terminology such as command parameter labels.
	osCode map[nodeID]string
	// piped marks identifiers written as |name| in the source.
	piped map[nodeID]bool
	// osEnum holds the enumeration type of typed-constant refs.
	osEnum map[nodeID]string
	// scopes is the stack of applications whose terminology is in scope,
	// innermost last (tell blocks, using terms from).
	scopes []string
	// targeted marks user handler calls with an explicit target (my foo:x,
	// x's foo:y, tell x to foo:y); only those use interleaved syntax; plain
	// calls keep the foo_bar_(x, y) form. curCall is the call being rendered.
	targeted map[nodeID]bool
	curCall  nodeID
	// contDepth counts the continuation-marked wrappers being rendered.
	contDepth int
	// inMy is set while rendering the operand of `my (…)`, whose
	// parentheses already delimit a targeted call's container.
	inMy bool
	// repeatWith counts enclosing `repeat with` loops, inside which Script
	// Editor parenthesizes a statement's targeted call: (x's foo:y).
	repeatWith int
	// enc is the encoding of the file's old byte strings.
	enc *mactext.Encoding
	// legacy is set for AppleScript 1.0 files (FASD before 1.01), and beta
	// for the 1.0b1 beta's (FASD 0.95).
	legacy, beta bool
	// classic is set for scripts from classic Mac OS: AppleScript 1.0 files,
	// and scripts read from a resource fork. They get classic terminology.
	classic bool
	// host is the application that runs the script, whose terms it uses
	// without a tell block, such as HyperCard for a script in a stack.
	host string
	// fromBytecode is set for output rebuilt from bytecode (run-only
	// scripts), which must compile back to the same code: reals keep every
	// digit, and a raw property code of the tell target is written
	// its «class x».
	fromBytecode bool
	// noUse is set for run-only output of a script with no use statement,
	// where Objective-C calls can't use the interleaved form.
	noUse bool
	// propertyRefs marks bytecode property references (MakeObjectAlias), as
	// against constant references the compiler folded into literals.
	propertyRefs map[nodeID]bool
	// cmdLabels holds the parameter labels of the command whose arguments
	// are being rendered; variables named like one need pipes.
	cmdLabels map[string]bool
	// noAdditions is set when the script's use statements leave out
	// `use scripting additions`; its identifiers then cannot clash with
	// Standard Additions terms, so those never force |pipes|.
	noAdditions bool
	// nesting counts active stmt/expr recursion (see maxNesting); active
	// and activeStmt hold the nodes being rendered, to break cycles.
	nesting            int
	helperDepth        int
	active, activeStmt map[nodeID]bool
	// hoisted marks 'l' nodes whose comment was moved to a header line.
	hoisted map[nodeID]bool
}

// Decompile reconstructs AppleScript source from the FASD parse tree.
func Decompile(f *File) string {
	if f.dc == nil {
		return f.source
	}
	if f.RunOnly() {
		src, err := DecompileBytecode(f)
		if err != nil {
			return fmt.Sprintf("-- run-only script; bytecode decompilation failed: %v\n", err) + src
		}
		return "-- Decompiled from run-only bytecode: comments and original formatting are not recoverable.\n\n" + src
	}
	return decompileWith(f.dc)
}

// decompileWith generates AppleScript source from the full parse tree.
func decompileWith(dc *decompState) string {
	var sb strings.Builder

	// The first node is the script's top-level block: handler definitions,
	// properties, statements and blank lines, in source order.
	// The id-0 wrapper record's second child is the script body. The legacy
	// scanner does not record wrappers; there the body is the first node.
	dc.noAdditions = dc.usesWithoutAdditions()
	dc.scopes = dc.usedApps() // `use application "Finder"` imports its terms everywhere
	if dc.host != "" {
		dc.scopes = append([]string{dc.host}, dc.scopes...)
	}
	var lines []string
	if root, ok := dc.wraps[0]; ok && len(root) > 1 && root[1] > 0 {
		lines = dc.block(root[1], 0)
	} else if len(dc.nodeOrder) > 0 && dc.nodes[dc.nodeOrder[0]].typ == 'k' {
		lines = dc.block(dc.nodeOrder[0], 0)
	}
	if len(lines) == 0 {
		lines = dc.topLevelStmts(nil)
	}
	lines = expandContinuations(lines)
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Node header flag bits.
const (
	flagAltSyntax      = 1 // alternate surface form (one-line tell/if, is in, …)
	flagPossessive     = 2 // 'n' written as `x's y` / `its y`
	flagParens         = 4 // 'l' is an explicitly parenthesized expression
	flagContinuation   = 8 // expression 'l': a `¬` line break precedes it
	flagNumericOrdinal = 2 // '4' written as 1st, 2nd, …
	// Statement 'l' nodes carry a comment kind in their low flag bits.
	// A statement 'l' node's low three flag bits give its comment style.
	commentKindMask   = 7
	commentBlock      = 0 // (* … *)
	commentLine       = 1 // -- …
	commentNone       = 2
	commentHeader     = 3     // -- … on the enclosing compound statement's header line
	commentHeaderBlk  = 4     // (* … *) on the header line
	commentHash       = 6     // # …
	commentHeaderHash = 7     // # … on the header line
	flagThe           = 0x100 // 'l': article "the"; 'n': "in" for "of"; '4': front/back
	// flagImplicitIts marks a bytecode `its x` where `its` is needed only
	// if x alone would name a class (not a flag compilers write).
	flagImplicitIts = 0x8000
)

// firstPositive returns the first positive child of n, or 0.
func firstPositive(n nodeRec) nodeID {
	for _, ch := range n.children {
		if ch > 0 {
			return ch
		}
	}
	return 0
}

// child0 returns the first child ref (possibly negative), or 0.
func child0(n nodeRec) nodeID {
	if len(n.children) == 0 {
		return 0
	}
	return n.children[0]
}

// child returns n.children[i] if present and positive, else 0.
func child(n nodeRec, i int) nodeID {
	if i < len(n.children) && n.children[i] > 0 {
		return n.children[i]
	}
	return 0
}
