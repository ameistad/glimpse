import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var settings: AppSettings
    @State private var testingConnection = false
    @State private var connectionStatus: ConnectionStatus = .unknown

    enum ConnectionStatus {
        case unknown
        case success(stats: Stats)
        case failure(error: String)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            GroupBox("Server") {
                VStack(alignment: .leading, spacing: 8) {
                    TextField("http://server:8080", text: $settings.serverURL)
                        .textFieldStyle(.roundedBorder)
                        .onSubmit { saveAndTest() }

                    HStack {
                        Button("Save & Connect") {
                            saveAndTest()
                        }
                        .disabled(testingConnection || settings.serverURL.isEmpty)

                        if testingConnection {
                            ProgressView()
                                .controlSize(.small)
                        }

                        Spacer()

                        switch connectionStatus {
                        case .unknown:
                            EmptyView()
                        case .success(let stats):
                            HStack(spacing: 4) {
                                Image(systemName: "checkmark.circle.fill")
                                    .foregroundColor(.green)
                                Text("\(stats.totalPhotos) photos")
                                    .foregroundColor(.secondary)
                            }
                        case .failure(let error):
                            HStack(spacing: 4) {
                                Image(systemName: "xmark.circle.fill")
                                    .foregroundColor(.red)
                                Text(error)
                                    .foregroundColor(.secondary)
                                    .lineLimit(1)
                            }
                        }
                    }
                }
                .padding(4)
            }

            GroupBox("Downloads") {
                VStack(alignment: .leading, spacing: 8) {
                    HStack {
                        TextField("Download path", text: $settings.downloadPath)
                            .textFieldStyle(.roundedBorder)

                        Button("Choose...") {
                            chooseDownloadPath()
                        }
                    }

                    Text("RAW files will be downloaded to this folder")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
                .padding(4)
            }

            Spacer()

            HStack {
                Text("Glimpse")
                    .fontWeight(.medium)
                Text("Version 1.0")
                    .foregroundColor(.secondary)
            }
            .font(.caption)
        }
        .frame(width: 420, height: 280)
        .padding()
    }

    private func saveAndTest() {
        testingConnection = true
        connectionStatus = .unknown
        settings.needsRefresh = true

        let client = APIClient(baseURL: settings.serverURL)

        Task {
            do {
                let stats = try await client.fetchStats()
                await MainActor.run {
                    connectionStatus = .success(stats: stats)
                    testingConnection = false
                }
            } catch {
                await MainActor.run {
                    connectionStatus = .failure(error: error.localizedDescription)
                    testingConnection = false
                }
            }
        }
    }

    private func chooseDownloadPath() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.canCreateDirectories = true

        if panel.runModal() == .OK, let url = panel.url {
            settings.downloadPath = url.path
        }
    }
}
