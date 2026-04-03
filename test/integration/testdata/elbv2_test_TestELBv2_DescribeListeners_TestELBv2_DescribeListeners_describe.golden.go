{
  "Listeners": [
    {
      "AlpnPolicy": null,
      "Certificates": null,
      "DefaultActions": [
        {
          "Type": "forward",
          "AuthenticateCognitoConfig": null,
          "AuthenticateOidcConfig": null,
          "FixedResponseConfig": null,
          "ForwardConfig": null,
          "JwtValidationConfig": null,
          "Order": null,
          "RedirectConfig": null,
          "TargetGroupArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/test-describe-listener-tg/a878fb28-e55c-465"
        }
      ],
      "ListenerArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:listener/app/test-describe-listener-lb/f1fbf09d-e33b-44e/bba16703-9b15-4f5",
      "LoadBalancerArn": "arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/app/test-describe-listener-lb/f1fbf09d-e33b-44e",
      "MutualAuthentication": null,
      "Port": 80,
      "Protocol": "HTTP",
      "SslPolicy": null
    }
  ],
  "NextMarker": null,
  "ResultMetadata": {}
}