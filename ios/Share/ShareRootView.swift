import SwiftUI

struct ShareRootView: View {
    var loader: () async throws -> SharedFile
    var complete: () -> Void

    @State private var message = "Sending to Trove…"
    @State private var finished = false

    var body: some View {
        VStack(spacing: 16) {
            Text(message)
                .multilineTextAlignment(.center)
                .padding()
            if finished {
                Button("Done", action: complete)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .task { await send() }
    }

    private func send() async {
        let config = IngestSettings.config
        guard config.isReady else {
            message = "Open Trove and paste the URL and token first."
            finished = true
            return
        }
        do {
            let file = try await loader()
            let response = try await IngestClient(config: config).upload(
                file: file.data,
                filename: file.filename,
                contentType: file.contentType
            )
            message = "Posted as \(response.type)."
        } catch IngestError.httpStatus(let status) {
            message = "Trove returned \(status)."
        } catch {
            message = "Could not send this share."
        }
        finished = true
    }
}
