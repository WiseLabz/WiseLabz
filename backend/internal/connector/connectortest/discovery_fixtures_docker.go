package connectortest

func init() {
	RegisterDiscoveryFixtures("docker",
		DiscoveryFixture{
			Name: "engine with server header", Port: 2375, Path: "/version", Match: true,
			Response: Response(200, `{"Platform":{"Name":"Docker Engine - Community"},"Components":[{"Name":"Engine","Version":"24.0.7"}],"Version":"24.0.7","ApiVersion":"1.43","MinAPIVersion":"1.12","Os":"linux","Arch":"amd64"}`,
				"Server", "Docker/24.0.7 (linux)", "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "engine without server header", Port: 2375, Path: "/version", Match: true,
			Response: Response(200, `{"Platform":{"Name":"Docker Engine - Community"},"Components":[{"Name":"Engine","Version":"24.0.7"}],"Version":"24.0.7","ApiVersion":"1.43","MinAPIVersion":"1.12","Os":"linux","Arch":"amd64"}`,
				"Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "podman compat api", Port: 2375, Path: "/version", Match: false,
			Response: Response(200, `{"Components":[{"Name":"Podman Engine","Version":"5.0.0"}],"ApiVersion":"1.41"}`,
				"Server", "Libpod/5.0.0 (linux)", "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "not found", Port: 2375, Path: "/version", Match: false,
			Response: Response(404, `{"message":"page not found"}`, "Content-Type", "application/json"),
		},
	)
}
