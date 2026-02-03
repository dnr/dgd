defaultArgs:

{
  src,
  vendorHash ? null,
  subPackage ? "", # relative to module, do not include "./" prefix
  pkgs ? defaultArgs.pkgs,
  go ? pkgs.go,
  # use nix 2.32.5 or higher for best results with dynamic derivations
  innerNix ? pkgs.nixVersions.nix_2_32,
  env ? { },
}:

let
  argenv = env;
  processBuild = pkgs.callPackage ./process-build/package.nix { };
  getModule = pkgs.callPackage ./getModule.nix { };
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

        splitmoddrv = pkgs.runCommandLocal "dgd-joined-deps.drv" {
          inherit modpkg innerNix;
          pkgsPath = pkgs.path; # TODO: avoid copying to store if already in store?
          outputHashMode = "text";
          requiredSystemFeatures = [ "recursive-nix" ];
          __contentAddressed = true; # why is this needed???
        } (builtins.readFile ./script/split.sh);
      in
      builtins.outputOf splitmoddrv.outPath "out";

  modname = getModule (src + "/go.mod") subPackage;

  dynDrv = pkgs.runCommand "dgd-${modname}-bin.drv" {
    inherit
      src
      go
      innerNix
      processBuild
      splitmodsjoin
      subPackage
      ;
    envJson = builtins.toJSON env;
    pkgsPath = pkgs.path; # TODO: avoid copying to store if already in store?
    outputHashMode = "text";
    requiredSystemFeatures = [ "recursive-nix" ];
    __contentAddressed = true; # needed to depend on a dynamic derivation??
  } (builtins.readFile ./script/build-n.sh);

  out = builtins.outputOf dynDrv.outPath "out";
in
out
