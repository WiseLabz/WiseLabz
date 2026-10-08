package connectortest

func init() {
	RegisterDiscoveryFixtures("caddy",
		DiscoveryFixture{
			Name: "config", Port: 2019, Path: "/config/", Match: true,
			Response: Response(200, `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"]}}}}}`,
				"Etag", `"/config/ 3f2a9c1e5b7d8a60"`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "config without etag", Port: 2019, Path: "/config/", Match: false,
			Response: Response(200, `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"]}}}}}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "weak etag", Port: 2019, Path: "/config/", Match: false,
			Response: Response(200, `{"apps":{}}`, "Etag", `W/"abc123"`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "etag for another path", Port: 2019, Path: "/config/", Match: false,
			Response: Response(200, `{"apps":{}}`, "Etag", `"/other/ 3f2a"`, "Content-Type", "application/json"),
		},
	)
}
