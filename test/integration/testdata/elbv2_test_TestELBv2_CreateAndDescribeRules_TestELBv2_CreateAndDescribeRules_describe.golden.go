{
  "NextMarker": null,
  "Rules": [
    {
      "Actions": [
        {
          "Type": "forward",
          "AuthenticateCognitoConfig": null,
          "AuthenticateOidcConfig": null,
          "FixedResponseConfig": null,
          "ForwardConfig": {
            "TargetGroupStickinessConfig": null,
            "TargetGroups": [
              {
                "TargetGroupArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/test-rules-tg/b15ce460-ae4d-460",
                "Weight": null
              }
            ]
          },
          "JwtValidationConfig": null,
          "Order": null,
          "RedirectConfig": null,
          "TargetGroupArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/test-rules-tg/b15ce460-ae4d-460"
        }
      ],
      "Conditions": [
        {
          "Field": "path-pattern",
          "HostHeaderConfig": null,
          "HttpHeaderConfig": null,
          "HttpRequestMethodConfig": null,
          "PathPatternConfig": {
            "RegexValues": null,
            "Values": [
              "/*"
            ]
          },
          "QueryStringConfig": null,
          "RegexValues": null,
          "SourceIpConfig": null,
          "Values": []
        }
      ],
      "IsDefault": false,
      "Priority": "1",
      "RuleArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:listener-rule/e58081c4-59ae-494/f53b3eb9-9457-465",
      "Transforms": null
    }
  ],
  "ResultMetadata": {}
}