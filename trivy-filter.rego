package trivy

default ignore = false

ignore {
    ignored_packages := {
        "stdlib"
    }
    ignored_packages[input.PkgName]
}

ignore {
    ignored_cves := {
        "GO-2026-5932" # golang.org/x/crypto deprecation of the obsolete package (openpgp)
    }
    ignored_cves[input.VulnerabilityID]
}
