package ws

import "github.com/maillunex-lang/CosmocRPG/server/internal/world"

// testRouter — маршрутизатор из одного главного мира для тестов (Э-1).
func testRouter() *world.Router {
	w := world.New()
	r := world.NewRouter(w)
	r.Add(w)
	return r
}
