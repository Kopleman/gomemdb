package command

type commandType string

const (
	CommandGET commandType = "GET"
	CommandSET commandType = "SET"
	CommandDEL commandType = "DEL"

	CommandUnknown commandType = "Unknown"
)

type Command struct {
	Type commandType

	Name string
	Set  SetArgs
}

type SetArgs struct {
	Value string
}
