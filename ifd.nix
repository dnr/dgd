defaultArgs:

{
  src,
  vendorHash ? null,
  subPackage ? "", # relative to module, do not include "./" prefix
  pkgs ? defaultArgs.pkgs,
  go ? pkgs.go,
  # 2.32 isn't needed here, but for consistency:
  innerNix ? pkgs.nixVersions.nix_2_32,
  env ? { },
}:

let
  argenv = env;
  processBuild = pkgs.callPackage ./process-build/package.nix { };
in
let
  proxyenv = argenv // {
    CGO_ENABLED = argenv.CGO_ENABLED or go.CGO_ENABLED;
  };
  env = proxyenv // {
    GOPROXY = "off";
  };

  splitmodsjoin =
    if vendorHash == null then
      null
    else
      let
        modpkg = pkgs.runCommand "dgd-deps" {
          inherit src go;
          env = proxyenv;
          outputHash = vendorHash;
          outputHashMode = "recursive";
        } (builtins.readFile ./script/download.sh);

        splitmodnix = pkgs.runCommandLocal "dgd-split-deps.nix" {
          inherit modpkg innerNix;
        } (builtins.readFile ./script/split.sh);
      in
      builtins.trace "importing splitmodnix: ${splitmodnix}" import splitmodnix {
        inherit pkgs modpkg;
      };

  nixexpr = pkgs.runCommand "dgd-drv.nix" {
    inherit
      src
      env
      go
      processBuild
      splitmodsjoin
      subPackage
      ;
  } (builtins.readFile ./script/build-n.sh);

  drv = builtins.trace "importing build: ${nixexpr}" import nixexpr {
    srcStr = builtins.toString src;
    goStr = builtins.toString go;
    envJson = builtins.toJSON env;
    pkgsPath = builtins.toString pkgs.path;
  };
in
drv
