# Custom recipe response fixtures

These JSON files are the recorded response bodies returned by the documented
recipe tests' local mock services. Their structure follows the service APIs:
[Sonarr's API](https://sonarr.tv/docs/api/) and the [Jellyfin `GetItems` request and response model](https://typescript-sdk.jellyfin.org/interfaces/generated-client.LibraryApiGetItemsRequest.html).
The Jellyfin authorization header format follows the [server's authorization parser](https://github.com/jellyfin/jellyfin/blob/master/Jellyfin.Server.Implementations/Security/AuthorizationContext.cs).

The fixtures are not copied from a real media library. Series and movie titles,
IDs, and filesystem paths are invented substitutions; no hostnames, usernames,
or credentials appear in the recorded bodies. Test-only tokens are supplied by
the Go tests and checked as request headers.
