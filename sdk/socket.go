package sdk

// TimecraftAddress is the socket that timecraft guests connect to in order to
// interact with the timecraft runtime on the host. Note that this is a
// virtual socket.
const TimecraftAddress = "127.0.0.1:3001"

// WorkAddress is the socket that receives work from the timecraft runtime.
// Note that this is a virtual socket.
const WorkAddress = "127.0.0.1:3000"
