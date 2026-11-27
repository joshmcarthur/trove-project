import SwiftUI
import UIKit
import UniformTypeIdentifiers

@objc(ShareViewController)
final class ShareViewController: UIViewController {
    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        let host = UIHostingController(
            rootView: ShareRootView(
                loader: loadSharedFile,
                complete: { [weak self] in
                    self?.extensionContext?.completeRequest(
                        returningItems: [],
                        completionHandler: nil
                    )
                }
            )
        )
        addChild(host)
        host.view.frame = view.bounds
        host.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        view.addSubview(host.view)
        host.didMove(toParent: self)
    }

    private func loadSharedFile() async throws -> SharedFile {
        guard
            let item = extensionContext?.inputItems.first as? NSExtensionItem,
            let provider = item.attachments?.first
        else {
            throw ShareLoadError.missingAttachment
        }
        let typeIdentifier = preferredType(provider)
        return try await withCheckedThrowingContinuation { continuation in
            provider.loadFileRepresentation(forTypeIdentifier: typeIdentifier) { url, error in
                if let error {
                    continuation.resume(throwing: error)
                    return
                }
                guard let url else {
                    continuation.resume(throwing: ShareLoadError.missingAttachment)
                    return
                }
                do {
                    let data = try Data(contentsOf: url)
                    let filename = url.lastPathComponent.isEmpty
                        ? "share.bin"
                        : url.lastPathComponent
                    let contentType = UTType(filenameExtension: url.pathExtension)?
                        .preferredMIMEType ?? "application/octet-stream"
                    continuation.resume(
                        returning: SharedFile(
                            data: data,
                            filename: filename,
                            contentType: contentType
                        )
                    )
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    private func preferredType(_ provider: NSItemProvider) -> String {
        if provider.hasItemConformingToTypeIdentifier(UTType.image.identifier) {
            return UTType.image.identifier
        }
        if provider.hasItemConformingToTypeIdentifier(UTType.fileURL.identifier) {
            return UTType.fileURL.identifier
        }
        if provider.hasItemConformingToTypeIdentifier(UTType.data.identifier) {
            return UTType.data.identifier
        }
        return provider.registeredTypeIdentifiers.first ?? UTType.data.identifier
    }
}

struct SharedFile {
    var data: Data
    var filename: String
    var contentType: String
}

enum ShareLoadError: Error {
    case missingAttachment
}
