package driver

var supportedVolumeTypes = map[string]struct{}{
	"apm":    {},
	"socket": {},
	"local":  {},
}

func isSupportedVolumeType(volumeType string) bool {
	_, found := supportedVolumeTypes[volumeType]
	return found
}

const SocketVolume = "socket"
const LocalVolume = "local"
const APM = "apm"
