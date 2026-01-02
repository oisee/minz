// Package dap implements the Debug Adapter Protocol for MinZ.
// See: https://microsoft.github.io/debug-adapter-protocol/
package dap

// Message is the base type for all DAP messages
type Message struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"` // "request", "response", "event"
}

// Request is a DAP request message
type Request struct {
	Message
	Command   string      `json:"command"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// Response is a DAP response message
type Response struct {
	Message
	RequestSeq   int         `json:"request_seq"`
	Success      bool        `json:"success"`
	Command      string      `json:"command"`
	ErrorMessage string      `json:"message,omitempty"`
	Body         interface{} `json:"body,omitempty"`
}

// Event is a DAP event message
type Event struct {
	Message
	Event string      `json:"event"`
	Body  interface{} `json:"body,omitempty"`
}

// === Capabilities ===

// Capabilities describes the debug adapter capabilities
type Capabilities struct {
	SupportsConfigurationDoneRequest bool `json:"supportsConfigurationDoneRequest"`
	SupportsFunctionBreakpoints      bool `json:"supportsFunctionBreakpoints"`
	SupportsConditionalBreakpoints   bool `json:"supportsConditionalBreakpoints"`
	SupportsHitConditionalBreakpoints bool `json:"supportsHitConditionalBreakpoints"`
	SupportsEvaluateForHovers        bool `json:"supportsEvaluateForHovers"`
	SupportsStepBack                 bool `json:"supportsStepBack"`
	SupportsSetVariable              bool `json:"supportsSetVariable"`
	SupportsRestartFrame             bool `json:"supportsRestartFrame"`
	SupportsGotoTargetsRequest       bool `json:"supportsGotoTargetsRequest"`
	SupportsStepInTargetsRequest     bool `json:"supportsStepInTargetsRequest"`
	SupportsCompletionsRequest       bool `json:"supportsCompletionsRequest"`
	SupportsModulesRequest           bool `json:"supportsModulesRequest"`
	SupportsRestartRequest           bool `json:"supportsRestartRequest"`
	SupportsExceptionOptions         bool `json:"supportsExceptionOptions"`
	SupportsValueFormattingOptions   bool `json:"supportsValueFormattingOptions"`
	SupportsExceptionInfoRequest     bool `json:"supportsExceptionInfoRequest"`
	SupportTerminateDebuggee         bool `json:"supportTerminateDebuggee"`
	SupportsDelayedStackTraceLoading bool `json:"supportsDelayedStackTraceLoading"`
	SupportsLoadedSourcesRequest     bool `json:"supportsLoadedSourcesRequest"`
	SupportsLogPoints                bool `json:"supportsLogPoints"`
	SupportsTerminateThreadsRequest  bool `json:"supportsTerminateThreadsRequest"`
	SupportsSetExpression            bool `json:"supportsSetExpression"`
	SupportsTerminateRequest         bool `json:"supportsTerminateRequest"`
	SupportsDataBreakpoints          bool `json:"supportsDataBreakpoints"`
	SupportsReadMemoryRequest        bool `json:"supportsReadMemoryRequest"`
	SupportsWriteMemoryRequest       bool `json:"supportsWriteMemoryRequest"`
	SupportsDisassembleRequest       bool `json:"supportsDisassembleRequest"`
	SupportsCancelRequest            bool `json:"supportsCancelRequest"`
	SupportsBreakpointLocationsRequest bool `json:"supportsBreakpointLocationsRequest"`
	SupportsClipboardContext         bool `json:"supportsClipboardContext"`
	SupportsSteppingGranularity      bool `json:"supportsSteppingGranularity"`
	SupportsInstructionBreakpoints   bool `json:"supportsInstructionBreakpoints"`
	SupportsExceptionFilterOptions   bool `json:"supportsExceptionFilterOptions"`
}

// === Request Arguments ===

// InitializeRequestArguments for initialize request
type InitializeRequestArguments struct {
	ClientID                     string `json:"clientID,omitempty"`
	ClientName                   string `json:"clientName,omitempty"`
	AdapterID                    string `json:"adapterID"`
	Locale                       string `json:"locale,omitempty"`
	LinesStartAt1                bool   `json:"linesStartAt1,omitempty"`
	ColumnsStartAt1              bool   `json:"columnsStartAt1,omitempty"`
	PathFormat                   string `json:"pathFormat,omitempty"`
	SupportsVariableType         bool   `json:"supportsVariableType,omitempty"`
	SupportsVariablePaging       bool   `json:"supportsVariablePaging,omitempty"`
	SupportsRunInTerminalRequest bool   `json:"supportsRunInTerminalRequest,omitempty"`
	SupportsMemoryReferences     bool   `json:"supportsMemoryReferences,omitempty"`
	SupportsProgressReporting    bool   `json:"supportsProgressReporting,omitempty"`
	SupportsInvalidatedEvent     bool   `json:"supportsInvalidatedEvent,omitempty"`
	SupportsMemoryEvent          bool   `json:"supportsMemoryEvent,omitempty"`
}

// LaunchRequestArguments for launch request
type LaunchRequestArguments struct {
	NoDebug      bool   `json:"noDebug,omitempty"`
	Program      string `json:"program"`
	Target       string `json:"target,omitempty"`       // "spectrum", "cpm", "cpc"
	StopOnEntry  bool   `json:"stopOnEntry,omitempty"`
	LoadAddress  int    `json:"loadAddress,omitempty"`
	StartAddress int    `json:"startAddress,omitempty"`
	SMCVisualization bool `json:"smcVisualization,omitempty"`
}

// SetBreakpointsArguments for setBreakpoints request
type SetBreakpointsArguments struct {
	Source      Source             `json:"source"`
	Breakpoints []SourceBreakpoint `json:"breakpoints,omitempty"`
	Lines       []int              `json:"lines,omitempty"`
	SourceModified bool            `json:"sourceModified,omitempty"`
}

// SourceBreakpoint is a breakpoint in source code
type SourceBreakpoint struct {
	Line         int    `json:"line"`
	Column       int    `json:"column,omitempty"`
	Condition    string `json:"condition,omitempty"`
	HitCondition string `json:"hitCondition,omitempty"`
	LogMessage   string `json:"logMessage,omitempty"`
}

// ContinueArguments for continue request
type ContinueArguments struct {
	ThreadId       int  `json:"threadId"`
	SingleThread   bool `json:"singleThread,omitempty"`
}

// NextArguments for next (step over) request
type NextArguments struct {
	ThreadId    int    `json:"threadId"`
	Granularity string `json:"granularity,omitempty"` // "statement", "line", "instruction"
	SingleThread bool  `json:"singleThread,omitempty"`
}

// StepInArguments for stepIn request
type StepInArguments struct {
	ThreadId    int    `json:"threadId"`
	TargetId    int    `json:"targetId,omitempty"`
	Granularity string `json:"granularity,omitempty"`
	SingleThread bool  `json:"singleThread,omitempty"`
}

// StepOutArguments for stepOut request
type StepOutArguments struct {
	ThreadId    int    `json:"threadId"`
	Granularity string `json:"granularity,omitempty"`
	SingleThread bool  `json:"singleThread,omitempty"`
}

// PauseArguments for pause request
type PauseArguments struct {
	ThreadId int `json:"threadId"`
}

// StackTraceArguments for stackTrace request
type StackTraceArguments struct {
	ThreadId   int `json:"threadId"`
	StartFrame int `json:"startFrame,omitempty"`
	Levels     int `json:"levels,omitempty"`
}

// ScopesArguments for scopes request
type ScopesArguments struct {
	FrameId int `json:"frameId"`
}

// VariablesArguments for variables request
type VariablesArguments struct {
	VariablesReference int    `json:"variablesReference"`
	Filter             string `json:"filter,omitempty"` // "indexed", "named"
	Start              int    `json:"start,omitempty"`
	Count              int    `json:"count,omitempty"`
}

// EvaluateArguments for evaluate request
type EvaluateArguments struct {
	Expression string `json:"expression"`
	FrameId    int    `json:"frameId,omitempty"`
	Context    string `json:"context,omitempty"` // "watch", "repl", "hover", "clipboard"
}

// ReadMemoryArguments for readMemory request
type ReadMemoryArguments struct {
	MemoryReference string `json:"memoryReference"`
	Offset          int    `json:"offset,omitempty"`
	Count           int    `json:"count"`
}

// WriteMemoryArguments for writeMemory request
type WriteMemoryArguments struct {
	MemoryReference string `json:"memoryReference"`
	Offset          int    `json:"offset,omitempty"`
	AllowPartial    bool   `json:"allowPartial,omitempty"`
	Data            string `json:"data"` // base64 encoded
}

// DisassembleArguments for disassemble request
type DisassembleArguments struct {
	MemoryReference   string `json:"memoryReference"`
	Offset            int    `json:"offset,omitempty"`
	InstructionOffset int    `json:"instructionOffset,omitempty"`
	InstructionCount  int    `json:"instructionCount"`
	ResolveSymbols    bool   `json:"resolveSymbols,omitempty"`
}

// === Response Bodies ===

// SetBreakpointsResponseBody for setBreakpoints response
type SetBreakpointsResponseBody struct {
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// ContinueResponseBody for continue response
type ContinueResponseBody struct {
	AllThreadsContinued bool `json:"allThreadsContinued,omitempty"`
}

// StackTraceResponseBody for stackTrace response
type StackTraceResponseBody struct {
	StackFrames []StackFrame `json:"stackFrames"`
	TotalFrames int          `json:"totalFrames,omitempty"`
}

// ScopesResponseBody for scopes response
type ScopesResponseBody struct {
	Scopes []Scope `json:"scopes"`
}

// VariablesResponseBody for variables response
type VariablesResponseBody struct {
	Variables []Variable `json:"variables"`
}

// EvaluateResponseBody for evaluate response
type EvaluateResponseBody struct {
	Result             string `json:"result"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
	MemoryReference    string `json:"memoryReference,omitempty"`
}

// ReadMemoryResponseBody for readMemory response
type ReadMemoryResponseBody struct {
	Address         string `json:"address"`
	UnreadableBytes int    `json:"unreadableBytes,omitempty"`
	Data            string `json:"data,omitempty"` // base64 encoded
}

// WriteMemoryResponseBody for writeMemory response
type WriteMemoryResponseBody struct {
	Offset       int `json:"offset,omitempty"`
	BytesWritten int `json:"bytesWritten,omitempty"`
}

// DisassembleResponseBody for disassemble response
type DisassembleResponseBody struct {
	Instructions []DisassembledInstruction `json:"instructions"`
}

// ThreadsResponseBody for threads response
type ThreadsResponseBody struct {
	Threads []Thread `json:"threads"`
}

// === Event Bodies ===

// StoppedEventBody for stopped event
type StoppedEventBody struct {
	Reason            string `json:"reason"` // "step", "breakpoint", "exception", "pause", "entry", "goto", "function breakpoint", "data breakpoint"
	Description       string `json:"description,omitempty"`
	ThreadId          int    `json:"threadId,omitempty"`
	PreserveFocusHint bool   `json:"preserveFocusHint,omitempty"`
	Text              string `json:"text,omitempty"`
	AllThreadsStopped bool   `json:"allThreadsStopped,omitempty"`
	HitBreakpointIds  []int  `json:"hitBreakpointIds,omitempty"`
}

// TerminatedEventBody for terminated event
type TerminatedEventBody struct {
	Restart interface{} `json:"restart,omitempty"`
}

// OutputEventBody for output event
type OutputEventBody struct {
	Category string `json:"category,omitempty"` // "console", "stdout", "stderr", "telemetry"
	Output   string `json:"output"`
	Group    string `json:"group,omitempty"` // "start", "startCollapsed", "end"
	VariablesReference int    `json:"variablesReference,omitempty"`
	Source   *Source `json:"source,omitempty"`
	Line     int     `json:"line,omitempty"`
	Column   int     `json:"column,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// === Types ===

// Source represents a source file
type Source struct {
	Name             string `json:"name,omitempty"`
	Path             string `json:"path,omitempty"`
	SourceReference  int    `json:"sourceReference,omitempty"`
	PresentationHint string `json:"presentationHint,omitempty"` // "normal", "emphasize", "deemphasize"
	Origin           string `json:"origin,omitempty"`
	AdapterData      interface{} `json:"adapterData,omitempty"`
}

// Breakpoint represents a breakpoint
type Breakpoint struct {
	Id        int     `json:"id,omitempty"`
	Verified  bool    `json:"verified"`
	Message   string  `json:"message,omitempty"`
	Source    *Source `json:"source,omitempty"`
	Line      int     `json:"line,omitempty"`
	Column    int     `json:"column,omitempty"`
	EndLine   int     `json:"endLine,omitempty"`
	EndColumn int     `json:"endColumn,omitempty"`
	InstructionReference string `json:"instructionReference,omitempty"`
	Offset    int     `json:"offset,omitempty"`
}

// StackFrame represents a stack frame
type StackFrame struct {
	Id                   int     `json:"id"`
	Name                 string  `json:"name"`
	Source               *Source `json:"source,omitempty"`
	Line                 int     `json:"line"`
	Column               int     `json:"column"`
	EndLine              int     `json:"endLine,omitempty"`
	EndColumn            int     `json:"endColumn,omitempty"`
	CanRestart           bool    `json:"canRestart,omitempty"`
	InstructionPointerReference string `json:"instructionPointerReference,omitempty"`
	ModuleId             interface{} `json:"moduleId,omitempty"`
	PresentationHint     string  `json:"presentationHint,omitempty"` // "normal", "label", "subtle"
}

// Scope represents a variable scope
type Scope struct {
	Name               string `json:"name"`
	PresentationHint   string `json:"presentationHint,omitempty"` // "arguments", "locals", "registers"
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
	Expensive          bool   `json:"expensive"`
	Source             *Source `json:"source,omitempty"`
	Line               int    `json:"line,omitempty"`
	Column             int    `json:"column,omitempty"`
	EndLine            int    `json:"endLine,omitempty"`
	EndColumn          int    `json:"endColumn,omitempty"`
}

// Variable represents a variable
type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	NamedVariables     int    `json:"namedVariables,omitempty"`
	IndexedVariables   int    `json:"indexedVariables,omitempty"`
	MemoryReference    string `json:"memoryReference,omitempty"`
	EvaluateName       string `json:"evaluateName,omitempty"`
}

// Thread represents a thread
type Thread struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// DisassembledInstruction represents a disassembled instruction
type DisassembledInstruction struct {
	Address          string  `json:"address"`
	InstructionBytes string  `json:"instructionBytes,omitempty"`
	Instruction      string  `json:"instruction"`
	Symbol           string  `json:"symbol,omitempty"`
	Location         *Source `json:"location,omitempty"`
	Line             int     `json:"line,omitempty"`
	Column           int     `json:"column,omitempty"`
	EndLine          int     `json:"endLine,omitempty"`
	EndColumn        int     `json:"endColumn,omitempty"`
}
