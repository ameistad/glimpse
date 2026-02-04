import SwiftUI

struct ContentView: View {
    @EnvironmentObject var settings: AppSettings
    @StateObject private var viewModel = PhotosViewModel()
    @State private var selectedPhoto: Photo?
    @State private var selectedFolder: Folder?
    @State private var searchText = ""
    @State private var showingError = false
    @State private var errorMessage = ""
    @State private var isScanning = false

    var body: some View {
        NavigationSplitView {
            SidebarView(
                folders: viewModel.folders,
                selectedFolder: $selectedFolder,
                stats: viewModel.stats
            )
            .frame(minWidth: 200)
        } content: {
            PhotoGridView(
                photos: filteredPhotos,
                selectedPhoto: $selectedPhoto,
                folderName: selectedFolder?.displayName ?? "All Photos",
                photoCount: filteredPhotos.count,
                onLoadMore: loadMorePhotos,
                thumbnailURL: { viewModel.apiClient?.thumbnailURL(for: $0) }
            )
            .frame(minWidth: 400)
        } detail: {
            if let photo = selectedPhoto {
                PhotoDetailView(
                    photo: photo,
                    thumbnailURL: viewModel.apiClient?.thumbnailURL(for: photo),
                    onDownload: { downloadPhoto(photo) }
                )
            } else {
                Text("Select a photo")
                    .foregroundColor(.secondary)
            }
        }
        .navigationTitle("Glimpse")
        .searchable(text: $searchText, prompt: "Filter photos")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button(action: refresh) {
                    Image(systemName: "arrow.clockwise")
                }
                .help("Refresh")
            }
            ToolbarItem(placement: .primaryAction) {
                Button(action: triggerRescan) {
                    if isScanning {
                        ProgressView()
                            .controlSize(.small)
                    } else {
                        Image(systemName: "arrow.triangle.2.circlepath")
                    }
                }
                .disabled(isScanning)
                .help("Rescan library for new photos")
            }
        }
        .onAppear {
            viewModel.configure(serverURL: settings.serverURL)
            refresh()
        }
        .onChange(of: settings.needsRefresh) { _, newValue in
            if newValue {
                settings.needsRefresh = false
                viewModel.configure(serverURL: settings.serverURL)
                refresh()
            }
        }
        .onChange(of: selectedFolder) { _, _ in
            viewModel.reset()
            loadMorePhotos()
        }
        .alert("Error", isPresented: $showingError) {
            Button("OK") { }
        } message: {
            Text(errorMessage)
        }
    }

    private var filteredPhotos: [Photo] {
        if searchText.isEmpty {
            return viewModel.photos
        }
        return viewModel.photos.filter { photo in
            photo.filename.localizedCaseInsensitiveContains(searchText) ||
            photo.folder.localizedCaseInsensitiveContains(searchText)
        }
    }

    private func refresh() {
        Task {
            do {
                try await viewModel.fetchFolders()
                try await viewModel.fetchStats()
                viewModel.reset()
                try await viewModel.fetchPhotos(folder: selectedFolder?.path)
            } catch {
                errorMessage = error.localizedDescription
                showingError = true
            }
        }
    }

    private func triggerRescan() {
        guard let apiClient = viewModel.apiClient else { return }
        isScanning = true
        Task {
            do {
                let status = try await apiClient.triggerScan()
                if status == "already_running" {
                    errorMessage = "A scan is already in progress."
                    showingError = true
                }
            } catch {
                errorMessage = "Failed to trigger scan: \(error.localizedDescription)"
                showingError = true
            }
            try? await Task.sleep(for: .seconds(3))
            refresh()
            isScanning = false
        }
    }

    private func loadMorePhotos() {
        Task {
            do {
                try await viewModel.fetchPhotos(folder: selectedFolder?.path)
            } catch {
                errorMessage = error.localizedDescription
                showingError = true
            }
        }
    }

    private func downloadPhoto(_ photo: Photo) {
        guard let apiClient = viewModel.apiClient else { return }

        let downloadURL = URL(fileURLWithPath: settings.downloadPath)

        Task {
            do {
                let savedURL = try await apiClient.downloadOriginal(photo, to: downloadURL)
                // Open in Finder
                NSWorkspace.shared.activateFileViewerSelecting([savedURL])
            } catch {
                errorMessage = "Download failed: \(error.localizedDescription)"
                showingError = true
            }
        }
    }
}

@MainActor
class PhotosViewModel: ObservableObject {
    @Published var photos: [Photo] = []
    @Published var folders: [Folder] = []
    @Published var stats: Stats?
    @Published var isLoading = false

    var apiClient: APIClient?
    var offset = 0
    private let limit = 100
    var hasMore = true

    func reset() {
        photos = []
        offset = 0
        hasMore = true
    }

    func configure(serverURL: String) {
        guard !serverURL.isEmpty, URL(string: serverURL) != nil else {
            apiClient = nil
            return
        }
        apiClient = APIClient(baseURL: serverURL)
    }

    func fetchPhotos(folder: String? = nil) async throws {
        guard let apiClient = apiClient, !isLoading, hasMore else { return }

        isLoading = true
        defer { isLoading = false }

        let newPhotos = try await apiClient.fetchPhotos(
            folder: folder,
            limit: limit,
            offset: offset
        )

        photos.append(contentsOf: newPhotos)
        offset += newPhotos.count
        hasMore = newPhotos.count == limit
    }

    func fetchFolders() async throws {
        guard let apiClient = apiClient else { return }
        folders = try await apiClient.fetchFolders()
    }

    func fetchStats() async throws {
        guard let apiClient = apiClient else { return }
        stats = try await apiClient.fetchStats()
    }
}
