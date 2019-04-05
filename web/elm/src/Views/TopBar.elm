module Views.TopBar exposing
    ( breadcrumbComponent
    , breadcrumbs
    , concourseLogo
    )

import Concourse
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , href
        , id
        )
import Message.Message exposing (Hoverable(..), Message(..))
import Routes
import Url
import Views.Styles as Styles


concourseLogo : Html Message
concourseLogo =
    Html.a (href "/" :: Styles.concourseLogo) []


breadcrumbs : Routes.Route -> Html Message
breadcrumbs route =
    Html.div
        (id "breadcrumbs" :: Styles.breadcrumbContainer)
    <|
        case route of
            Routes.Pipeline { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                ]

            Routes.Build { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , jobBreadcrumb id.jobName
                ]

            Routes.Resource { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , resourceBreadcrumb id.resourceName
                ]

            Routes.Job { id } ->
                [ pipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , jobBreadcrumb id.jobName
                ]

            _ ->
                []


breadcrumbComponent : String -> String -> List (Html Message)
breadcrumbComponent componentType name =
    [ Html.div
        (Styles.breadcrumbComponent componentType)
        []
    , Html.text <| decodeName name
    ]


breadcrumbSeparator : Html Message
breadcrumbSeparator =
    Html.li
        (class "breadcrumb-separator" :: Styles.breadcrumbItem False)
        [ Html.text "/" ]


pipelineBreadcrumb : Concourse.PipelineIdentifier -> Html Message
pipelineBreadcrumb pipelineId =
    Html.a
        ([ id "breadcrumb-pipeline"
         , href <|
            Routes.toString <|
                Routes.Pipeline { id = pipelineId, groups = [] }
         ]
            ++ Styles.breadcrumbItem True
        )
        (breadcrumbComponent "pipeline" pipelineId.pipelineName)


jobBreadcrumb : String -> Html Message
jobBreadcrumb jobName =
    Html.li
        (id "breadcrumb-job" :: Styles.breadcrumbItem False)
        (breadcrumbComponent "job" jobName)


resourceBreadcrumb : String -> Html Message
resourceBreadcrumb resourceName =
    Html.li
        (id "breadcrumb-resource" :: Styles.breadcrumbItem False)
        (breadcrumbComponent "resource" resourceName)


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Url.percentDecode name)
