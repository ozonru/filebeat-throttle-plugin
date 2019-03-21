package plugin

import (
	"github.com/elastic/beats/libbeat/processors"
	"github.com/ozonru/filebeat-throttle-plugin"
)

var Bundle = processors.Plugin("throttle", throttleplugin.NewProcessor)
