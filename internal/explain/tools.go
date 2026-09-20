package explain

import "github.com/Dhafer84/firstgreenci/internal/i18n"

// The guides below are per system, because "install Docker" means a different
// sequence of gestures on each one, and a beginner needs the gestures.

// DockerInstallGuide explains how to install Docker on the running system.
func DockerInstallGuide(catalog *i18n.Catalog, goos string) string {
	return catalog.T("install.docker." + systemKey(goos))
}

// DockerStartGuide explains how to start Docker once it is installed. It is
// the most common situation: Docker is there, the application is closed.
func DockerStartGuide(catalog *i18n.Catalog, goos string) string {
	return catalog.T("start.docker." + systemKey(goos))
}

// ActInstallGuide explains how to install act on the running system.
func ActInstallGuide(catalog *i18n.Catalog, goos string) string {
	return catalog.T("install.act." + systemKey(goos))
}

// systemKey maps a Go system name to the suffix used in the catalogs. Every
// system that is neither macOS nor Windows is treated as Linux, which is what
// the remaining ones are in practice.
func systemKey(goos string) string {
	switch goos {
	case "darwin", "windows":
		return goos
	default:
		return "linux"
	}
}
