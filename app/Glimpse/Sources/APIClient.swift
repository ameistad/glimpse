import Foundation

class APIClient: ObservableObject {
    private let baseURL: String
    private let session: URLSession
    private let decoder: JSONDecoder

    init(baseURL: String) {
        self.baseURL = baseURL.trimmingCharacters(in: CharacterSet(charactersIn: "/"))

        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        config.timeoutIntervalForResource = 300
        self.session = URLSession(configuration: config)

        self.decoder = JSONDecoder()
        self.decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let dateString = try container.decode(String.self)

            // Try multiple date formats
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

            // Fallback to ISO8601
            if let date = ISO8601DateFormatter().date(from: dateString) {
                return date
            }

            throw DecodingError.dataCorruptedError(
                in: container,
                debugDescription: "Cannot decode date: \(dateString)"
            )
        }
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

        let (data, _) = try await session.data(from: components.url!)
        return try decoder.decode([Photo].self, from: data)
    }

    func fetchFolders() async throws -> [Folder] {
        let url = URL(string: "\(baseURL)/api/folders")!
        let (data, _) = try await session.data(from: url)
        return try decoder.decode([Folder].self, from: data)
    }

    func fetchStats() async throws -> Stats {
        let url = URL(string: "\(baseURL)/api/stats")!
        let (data, _) = try await session.data(from: url)
        return try decoder.decode(Stats.self, from: data)
    }

    func thumbnailURL(for photo: Photo) -> URL {
        URL(string: "\(baseURL)/api/photos/\(photo.id)/thumbnail")!
    }

    func originalURL(for photo: Photo) -> URL {
        URL(string: "\(baseURL)/api/photos/\(photo.id)/original")!
    }

    func downloadOriginal(_ photo: Photo, to directory: URL) async throws -> URL {
        let url = originalURL(for: photo)
        let (tempURL, _) = try await session.download(from: url)

        let destination = directory.appendingPathComponent(photo.filename)

        // Remove existing file if present
        try? FileManager.default.removeItem(at: destination)

        try FileManager.default.moveItem(at: tempURL, to: destination)
        return destination
    }
}
