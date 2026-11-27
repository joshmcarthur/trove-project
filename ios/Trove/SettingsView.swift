import SwiftUI

struct SettingsView: View {
    @State private var baseURL: String
    @State private var token: String
    @State private var saved = false

    init(config: IngestConfig = IngestSettings.config) {
        _baseURL = State(initialValue: config.baseURL)
        _token = State(initialValue: config.token)
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("https://trove.example.ts.net", text: $baseURL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                    SecureField("write:capture token", text: $token)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                } header: {
                    Text("Ingest")
                } footer: {
                    Text(
                        "Paste the Trove URL with no trailing slash, then the token from manage.py token create. After that, share a photo or file and choose Trove."
                    )
                }
                Button("Save") {
                    IngestSettings.save(IngestConfig(baseURL: baseURL, token: token))
                    saved = true
                }
                .disabled(!IngestConfig(baseURL: baseURL, token: token).isReady)
                if saved {
                    Text("Saved.")
                }
            }
            .navigationTitle("Trove")
        }
    }
}
