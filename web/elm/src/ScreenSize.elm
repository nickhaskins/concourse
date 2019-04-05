module ScreenSize exposing (ScreenSize(..), fromWindowSize)


type ScreenSize
    = Phone
    | Tablet
    | Desktop
    | BigDesktop


fromWindowSize : Float -> ScreenSize
fromWindowSize width =
    if width < 361 then
        Phone

    else if width < 812 then
        Tablet

    else if width < 1230 then
        Desktop

    else
        BigDesktop
