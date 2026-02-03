{ lib }:
let
  # Replaces all characters besides alphanumeric, dash, and underscore with dash.
  # This should match toNixName in process-build/generator.go
  sanitize =
    name:
    builtins.concatStringsSep "" (
      map (p: if builtins.isList p then "-" else p) (builtins.split "[^a-zA-Z0-9_-]+" name)
    );

  # Takes a path to a go.mod file and subpackage and returns sanitized module name
  parseGoMod =
    path: subPackage:
    let
      content = builtins.readFile path;
      lines = builtins.filter (s: builtins.isString s && s != "") (builtins.split "\n" content);
      isModuleLine = line: builtins.match "^module[[:space:]]+.*" line != null;
      moduleLine =
        lib.findFirst isModuleLine
          (builtins.throw "go.mod at ${toString path} does not contain a 'module' directive")
          lines;
      match = builtins.match "^module[[:space:]]+([^[:space:]]+).*" moduleLine;
      mod = builtins.head match;
      package = if subPackage == "" then mod else mod + "/" + subPackage;
    in
    sanitize package;
in
parseGoMod
