package main

import "runtime/debug"

// moduleVersion is a dependency's version as the build recorded it; with a
// go.work replacement it is "(devel)", and the replacement's path is added.
func moduleVersion(module string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path != module {
			continue
		}
		if dep.Replace != nil {
			return dep.Version + " => " + dep.Replace.Path + " " + dep.Replace.Version
		}
		return dep.Version
	}
	return "unknown"
}
