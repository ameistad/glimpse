import SwiftUI

struct AuthenticatedImage: View {
    let client: APIClient?
    let url: URL?
    let width: CGFloat?
    let height: CGFloat?

    @State private var image: NSImage?
    @State private var failed = false
    @State private var taskID = UUID()

    var body: some View {
        Group {
            if let image {
                Image(nsImage: image)
                    .resizable()
                    .aspectRatio(contentMode: .fill)
            } else if failed {
                Image(systemName: "photo")
                    .font(.largeTitle)
                    .foregroundColor(.secondary)
            } else {
                ProgressView()
            }
        }
        .frame(width: width, height: height)
        .clipped()
        .task(id: taskID) {
            await loadImage()
        }
        .onChange(of: url) { _, _ in
            image = nil
            failed = false
            taskID = UUID()
        }
    }

    private func loadImage() async {
        guard let client, let url else {
            failed = true
            return
        }
        do {
            image = try await client.fetchImage(url)
        } catch {
            failed = true
        }
    }
}
