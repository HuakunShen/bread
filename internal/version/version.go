package version

// Value is replaced by the release workflow through go build -ldflags.
var Value = "dev"

// Repository is the public GitHub repository used for release discovery.
const Repository = "HuakunShen/bread"
