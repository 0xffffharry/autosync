package core

import "autosync/util"

type CoreOptions struct {
	Dir          string                `json:"dir"`
	RclonePath   string                `json:"rclone_path,omitempty"`   // Or ENV RCLONE_PATH
	RcloneConfig string                `json:"rclone_config,omitempty"` // Or ENV RCLONE_CONFIG
	Arg          util.Listable[string] `json:"arg,omitempty"`
	FilterRule   util.Listable[string] `json:"filter_rule,omitempty"` // or
	FilterMode   string                `json:"filter_mode,omitempty"` // exclude | include
	RemotePath   util.Listable[string] `json:"remote_path"`
}
