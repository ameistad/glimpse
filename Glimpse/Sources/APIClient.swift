import Foundation
import AppKit

class InsecureSessionDelegate: NSObject, URLSessionDelegate {
    func urlSession(_ session: URLSession, didReceive challenge: URLAuthenticationChallenge) async -> (URLSession.AuthChallengeDisposition, URLCredential?) {
        if challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust,
           let trust = challenge.protectionSpace.serverTrust {
            return (.useCredential, URLCredential(trust: trust))
        }
        return (.performDefaultHandling, nil)
    }
}

class APIClient: ObservableObject {
    private let baseURL: String
    private let apiKey: String
    let session: URLSession
    private let decoder: JSONDecoder
    private let sessionDelegate = InsecureSessionDelegate()

    init(baseURL: String, apiKey: String) {
        self.baseURL = baseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        self.apiKey = apiKey

        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        config.timeoutIntervalForResource = 300
        self.session = URLSession(configuration: config, delegate: sessionDelegate, delegateQueue: nil)

        self.decoder = JSONDecoder()
        self.decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let dateString = try container.decode(String.self)

            let formatters = [
                "yyyy-MM-dd'T'HH:mm:ss.SSSSSSSSSZ",
                "yyyy-MM-dd'T'HH:mm:ss.SSSSSSZ",
                "yyyy-MM-dd'T'HH:mm:ss.SSSZ",
                "yyyy-MM-dd'T'HH:mm:ssZ",
                "yyyy-MM-dd HH:mm:ss"
            ]

            for format in formatters {
                let formatter = DateFormatter()
                formatter.dateFormat = format
                formatter.locale = Locale(identifier: "en_US_POSIX")
                formatter.timeZone = TimeZone(secondsFromGMT: 0)

                if let date = formatter.date(from: dateString) {
                    return date
                }
            }

            if let date = ISO8601DateFormatter().date(from: dateString) {
                return date
            }

            throw DecodingError.dataCorruptedError(
                in: container,
                debugDescription: "Cannot decode date: \(dateString)"
            )
        }
    }

    private func authenticatedRequest(for url: URL) -> URLRequest {
        var request = URLRequest(url: url)
        request.setValue(apiKey, forHTTPHeaderField: "X-API-Key")
        return request
    }

    func fetchPhotos(folder: String? = nil, limit: Int = 100, offset: Int = 0) async throws -> [Photo] {
        var components = URLComponents(string: "\(baseURL)/api/photos")!
        var queryItems = [
            URLQueryItem(name: "limit", value: String(limit)),
            URLQueryItem(name: "offset", value: String(offset))
        ]
        if let folder = folder, !folder.isEmpty {
            queryItems.append(URLQueryItem(name: "folder", value: folder))
        }
        components.queryItems = queryItems

        let request = authenticatedRequest(for: components.url!)
        let (data, _) = try await session.data(for: request)
        return try decoder.decode([Photo].self, from: data)
    }

    func fetchFolders() async throws -> [Folder] {
        let url = URL(string: "\(baseURL)/api/folders")!
        let request = authenticatedRequest(for: url)
        let (data, _) = try await session.data(for: request)
        return try decoder.decode([Folder].self, from: data)
    }

    func fetchStats() async throws -> Stats {
        let url = URL(string: "\(baseURL)/api/stats")!
        let request = authenticatedRequest(for: url)
        let (data, _) = try await session.data(for: request)
        return try decoder.decode(Stats.self, from: data)
    }

    func thumbnailURL(for photo: Photo) -> URL {
        URL(string: "\(baseURL)/api/photos/\(photo.id)/thumbnail")!
    }

    func originalURL(for photo: Photo) -> URL {
        URL(string: "\(baseURL)/api/photos/\(photo.id)/original")!
    }

    func streamURL(for photo: Photo) -> URL {
        URL(string: "\(baseURL)/api/photos/\(photo.id)/stream")!
    }

    func triggerScan() async throws -> String {
        let url = URL(string: "\(baseURL)/api/scan")!
        var request = authenticatedRequest(for: url)
        request.httpMethod = "POST"
        let (data, _) = try await session.data(for: request)
        let response = try JSONDecoder().decode([String: String].self, from: data)
        return response["status"] ?? "unknown"
    }

    func fetchImage(_ url: URL) async throws -> NSImage {
        let request = authenticatedRequest(for: url)
        let (data, _) = try await session.data(for: request)
        guard let image = NSImage(data: data) else {
            throw URLError(.cannotDecodeContentData)
        }
        return image
    }

    func downloadOriginal(_ photo: Photo, to directory: URL) async throws -> URL {
        let url = originalURL(for: photo)
        let request = authenticatedRequest(for: url)
        let (tempURL, _) = try await session.download(for: request)

        let destination = directory.appendingPathComponent(photo.filename)
        try? FileManager.default.removeItem(at: destination)
        try FileManager.default.moveItem(at: tempURL, to: destination)
        return destination
    }
}
