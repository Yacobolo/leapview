terraform {
  cloud {
    organization = "Flid"

    workspaces {
      name = "leapview-site-production"
    }
  }
}
