import XCTest

final class IngestClientTests: XCTestCase {
    func testBuildsMultipartPostToIngest() throws {
        let client = IngestClient(
            config: IngestConfig(
                baseURL: "https://trove.example.ts.net",
                token: "secret"
            )
        )
        let request = try client.makeRequest(
            file: Data("png".utf8),
            filename: "x.png",
            contentType: "image/png"
        )
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertEqual(
            request.url?.absoluteString,
            "https://trove.example.ts.net/ingest/"
        )
        XCTAssertEqual(
            request.value(forHTTPHeaderField: "Authorization"),
            "Bearer secret"
        )
        let body = String(decoding: request.httpBody ?? Data(), as: UTF8.self)
        XCTAssertTrue(body.contains("name=\"file\""))
        XCTAssertTrue(body.contains("filename=\"x.png\""))
        XCTAssertTrue(body.contains("name=\"title\""))
        XCTAssertTrue(body.contains("name=\"notes\""))
        XCTAssertTrue(body.contains("name=\"url\""))
    }

    func testStripsTrailingSlash() throws {
        let client = IngestClient(
            config: IngestConfig(
                baseURL: "https://trove.example.ts.net/",
                token: "t"
            )
        )
        let request = try client.makeRequest(
            file: Data("x".utf8),
            filename: "a.bin",
            contentType: "application/octet-stream"
        )
        XCTAssertEqual(
            request.url?.absoluteString,
            "https://trove.example.ts.net/ingest/"
        )
    }

    func testParse201() throws {
        let client = IngestClient(
            config: IngestConfig(baseURL: "https://x.test", token: "t")
        )
        let json = Data(#"{"id":"abc","type":"capture.image"}"#.utf8)
        let response = try client.parseResponse(data: json, statusCode: 201)
        XCTAssertEqual(response.id, "abc")
        XCTAssertEqual(response.type, "capture.image")
    }

    func testEmptyFileAndMissingConfigFail() {
        let ready = IngestClient(
            config: IngestConfig(baseURL: "https://x.test", token: "t")
        )
        XCTAssertThrowsError(
            try ready.makeRequest(
                file: Data(),
                filename: "x.png",
                contentType: "image/png"
            )
        ) { error in
            XCTAssertEqual(error as? IngestError, .emptyFile)
        }

        let missing = IngestClient(config: IngestConfig(baseURL: "", token: ""))
        XCTAssertThrowsError(
            try missing.makeRequest(
                file: Data("x".utf8),
                filename: "x.png",
                contentType: "image/png"
            )
        ) { error in
            XCTAssertEqual(error as? IngestError, .notConfigured)
        }
    }
}
