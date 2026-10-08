package connectortest

func init() {
	RegisterDiscoveryFixtures("portainer",
		DiscoveryFixture{
			Name: "system status", Port: 9443, Path: "/api/system/status", Match: true,
			Response: Response(200, `{"Version":"2.21.4","InstanceID":"299ab403-70a8-4c05-92f7-bf7a994d50df"}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "traefik on the same port", Port: 9443, Path: "/api/system/status", Match: false,
			Response: Response(200, `{"Version":"3.1.2","Codename":"comte","startDate":"2024-09-10T10:00:00Z"}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "not found", Port: 9443, Path: "/api/system/status", Match: false,
			Response: Response(404, `{"message":"Not Found"}`, "Content-Type", "application/json"),
		},
	)
}
