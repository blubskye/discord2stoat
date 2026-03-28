package tui

// msgConfirmed is sent when the user presses Confirm on Screen 1.
type msgConfirmed struct{}

// msgQuit is sent when the user presses Quit.
type msgQuit struct{}

// msgStartClone is sent when the user presses Start on Screen 2.
type msgStartClone struct{}

// msgBack is sent when the user presses Back on Screen 2.
type msgBack struct{}

// msgPipelineDone is sent when the pipeline progress channel is closed.
type msgPipelineDone struct{}
