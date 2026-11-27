import Foundation

struct IngestConfig: Equatable {
    var baseURL: String
    var token: String

    var ingestURL: URL? {
        let trimmed = baseURL.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }
        let root = trimmed.hasSuffix("/") ? String(trimmed.dropLast()) : trimmed
        return URL(string: root + "/ingest/")
    }

    var isReady: Bool {
        ingestURL != nil
            && !token.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}

struct IngestResponse: Equatable {
    var id: String
    var type: String
}

enum IngestError: Error, Equatable {
    case notConfigured
    case emptyFile
    case httpStatus(Int)
    case badResponse
}

struct IngestClient {
    var config: IngestConfig

    func makeRequest(
        file: Data,
        filename: String,
        contentType: String,
        title: String = "",
        notes: String = "",
        pageURL: String = ""
    ) throws -> URLRequest {
        guard config.isReady, let url = config.ingestURL else {
            throw IngestError.notConfigured
        }
        guard !file.isEmpty else {
            throw IngestError.emptyFile
        }

        let token = config.token.trimmingCharacters(in: .whitespacesAndNewlines)
        let boundary = "Boundary-\(UUID().uuidString)"
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue(
            "multipart/form-data; boundary=\(boundary)",
            forHTTPHeaderField: "Content-Type"
        )
        request.httpBody = multipartBody(
            boundary: boundary,
            file: file,
            filename: filename,
            contentType: contentType,
            title: title,
            notes: notes,
            pageURL: pageURL
        )
        return request
    }

    func parseResponse(data: Data, statusCode: Int) throws -> IngestResponse {
        guard statusCode == 201 else {
            throw IngestError.httpStatus(statusCode)
        }
        guard
            let json = try JSONSerialization.jsonObject(with: data) as? [String: Any],
            let id = json["id"] as? String,
            let type = json["type"] as? String
        else {
            throw IngestError.badResponse
        }
        return IngestResponse(id: id, type: type)
    }

    func upload(
        file: Data,
        filename: String,
        contentType: String,
        session: URLSession = .shared
    ) async throws -> IngestResponse {
        let request = try makeRequest(
            file: file,
            filename: filename,
            contentType: contentType
        )
        let (data, response) = try await session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        return try parseResponse(data: data, statusCode: status)
    }

    private func multipartBody(
        boundary: String,
        file: Data,
        filename: String,
        contentType: String,
        title: String,
        notes: String,
        pageURL: String
    ) -> Data {
        var body = Data()
        func append(_ text: String) {
            body.append(Data(text.utf8))
        }
        func field(_ name: String, _ value: String) {
            append("--\(boundary)\r\n")
            append("Content-Disposition: form-data; name=\"\(name)\"\r\n\r\n")
            append("\(value)\r\n")
        }

        append("--\(boundary)\r\n")
        append(
            "Content-Disposition: form-data; name=\"file\"; filename=\"\(filename)\"\r\n"
        )
        append("Content-Type: \(contentType)\r\n\r\n")
        body.append(file)
        append("\r\n")
        field("title", title)
        field("notes", notes)
        field("url", pageURL)
        append("--\(boundary)--\r\n")
        return body
    }
}
