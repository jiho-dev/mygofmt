//go:build linux

package sample

import (
	sampletypes "example.com/sample/types"
	"fmt"
)

//go:noinline
func KeepNames() {
	value := struct {
		PipelineId string `json:"pipeline_id"`
		Id         string `json:"id"`
	}{PipelineId: "an intentionally long pipeline identifier", Id: "identifier"}

	fmt.Printf("%s", value.Id)

	_ = sampletypes.Host{}
}
