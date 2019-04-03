module Dashboard.DashboardPreview exposing (view)

import Colors
import Concourse
import Concourse.BuildStatus
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import Dashboard.Styles as Styles
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href, style)
import List.Extra exposing (find)
import Routes
import Time
import TopologicalSort exposing (flattenToLayers)


view : List Concourse.Job -> Html msg
view jobs =
    let
        jobDependencies : Concourse.Job -> List Concourse.Job
        jobDependencies job =
            job.inputs
                |> List.concatMap .passed
                |> List.filterMap (\name -> find (\j -> j.name == name) jobs)

        layers : List (List Concourse.Job)
        layers =
            flattenToLayers (List.map (\j -> ( j, jobDependencies j )) jobs)

        width : Int
        width =
            List.length layers

        height : Int
        height =
            layers
                |> List.map List.length
                |> List.maximum
                |> Maybe.withDefault 0
    in
    Html.div
        [ classList
            [ ( "pipeline-grid", True )
            , ( "pipeline-grid-wide", width > 12 )
            , ( "pipeline-grid-tall", height > 12 )
            , ( "pipeline-grid-super-wide", width > 24 )
            , ( "pipeline-grid-super-tall", height > 24 )
            ]
        ]
        (List.map viewJobLayer layers)


viewJobLayer : List Concourse.Job -> Html msg
viewJobLayer jobs =
    Html.div [ class "parallel-grid" ] (List.map viewJob jobs)


viewJob : Concourse.Job -> Html msg
viewJob job =
    let
        latestBuild : Maybe Concourse.Build
        latestBuild =
            if job.nextBuild == Nothing then
                job.finishedBuild

            else
                job.nextBuild

        buildRoute : Routes.Route
        buildRoute =
            case latestBuild of
                Nothing ->
                    Routes.jobRoute job

                Just build ->
                    Routes.buildRoute build
    in
    Html.div
        (attribute "data-tooltip" job.name :: jobStyle job)
        [ Html.a
            [ href <| Routes.toString buildRoute
            , style "flex-grow" "1"
            ]
            [ Html.text "" ]
        ]


jobStyle : Concourse.Job -> List (Html.Attribute msg)
jobStyle job =
    [ style "flex-grow" "1"
    , style "display" "flex"
    , style "margin" "2px"
    ]
        ++ (if job.paused then
                [ style "background-color" <|
                    Colors.statusColor PipelineStatusPaused
                ]

            else
                let
                    finishedBuildStatus =
                        job.finishedBuild
                            |> Maybe.map .status
                            |> Maybe.withDefault Concourse.BuildStatusPending

                    isRunning =
                        job.nextBuild /= Nothing

                    color =
                        Colors.buildStatusColor True finishedBuildStatus
                in
                Styles.texture "pipeline-running" isRunning color
           )
