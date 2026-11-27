import Foundation

enum IngestSettings {
    static let appGroup = "group.com.joshmcarthur.trove"
    static let baseURLKey = "ingestBaseURL"
    static let tokenKey = "ingestToken"

    static var store: UserDefaults {
        UserDefaults(suiteName: appGroup) ?? .standard
    }

    static var config: IngestConfig {
        IngestConfig(
            baseURL: store.string(forKey: baseURLKey) ?? "",
            token: store.string(forKey: tokenKey) ?? ""
        )
    }

    static func save(_ config: IngestConfig) {
        store.set(config.baseURL, forKey: baseURLKey)
        store.set(config.token, forKey: tokenKey)
    }
}
