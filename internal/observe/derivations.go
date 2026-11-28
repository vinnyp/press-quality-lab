package observe

// SystemNamed compares the product version exactly; the build is not a version.
func SystemNamed(product string) bool { return product == "27.0.1" || product == "27.2" }

// PlantedFacts derives the two independently recorded read-proof signals.
func PlantedFacts(before, after FileFacts) Planted {
	return Planted{AtimeMoved: after.Access.After(before.Access), SameFile: before.Device == after.Device && before.Inode == after.Inode}
}
