{
  mkShell,
  scid,
  nixfmt,
  gopls,
}:

mkShell {
  inputsFrom = [ scid ];

  buildInputs = [
    gopls
    nixfmt
  ];

  shellHook = ''
    export PS1="\033[0;31m[scid]\033[0m $PS1"
  '';
}
