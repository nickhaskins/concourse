module Common exposing (context, describe, iconSelector, it, queryView)

import Application.Application as Application
import Expect exposing (Expectation)
import Html
import Message.TopLevelMessage exposing (TopLevelMessage)
import Test exposing (Test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (Selector, style)


queryView : Application.Model -> Query.Single TopLevelMessage
queryView =
    Application.view
        >> .body
        >> List.head
        >> Maybe.withDefault (Html.text "")
        >> Query.fromHtml


describe : String -> model -> List (model -> Test) -> Test
describe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


context : String -> (a -> b) -> List (b -> Test) -> (a -> Test)
context description setup subTests beforeEach =
    Test.describe description
        (subTests |> List.map (\f -> f <| setup beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


iconSelector : { size : String, image : String } -> List Selector
iconSelector { size, image } =
    [ style "background-image" <| "url(/public/images/" ++ image ++ ")"
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "width" size
    , style "height" size
    ]
