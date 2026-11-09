package world

// StarterEngineInstanceID is the persistent identity of the existing bound
// Strizh engine. Reuse it when materializing ownership; do not issue a new ID
// on reconnect, migration or loadout creation.
func StarterEngineInstanceID(owner string) string { return starterEngineID(owner) }
