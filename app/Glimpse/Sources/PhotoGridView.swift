import SwiftUI

struct PhotoGridView: View {
    let photos: [Photo]
    @Binding var selectedPhoto: Photo?
    let folderName: String
    let photoCount: Int
    let onLoadMore: () -> Void
    let apiClient: APIClient?
    let thumbnailURL: (Photo) -> URL?

    @State private var gridSize: CGFloat = 150

    private let columns = [
        GridItem(.adaptive(minimum: 120, maximum: 300), spacing: 8)
    ]

    var body: some View {
        ScrollView {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(folderName)
                        .font(.title2)
                        .fontWeight(.semibold)
                    Text("\(photoCount) photos")
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
                Spacer()
            }
            .padding(.horizontal)
            .padding(.top, 12)
            .padding(.bottom, 4)

            LazyVGrid(columns: columns, spacing: 8) {
                ForEach(photos) { photo in
                    PhotoThumbnail(
                        photo: photo,
                        client: apiClient,
                        url: thumbnailURL(photo),
                        isSelected: selectedPhoto?.id == photo.id
                    )
                    .onTapGesture {
                        selectedPhoto = photo
                    }
                    .onAppear {
                        if photo.id == photos.last?.id {
                            onLoadMore()
                        }
                    }
                }
            }
            .padding()
        }
        .background(Color(NSColor.controlBackgroundColor))
    }
}

struct PhotoThumbnail: View {
    let photo: Photo
    let client: APIClient?
    let url: URL?
    let isSelected: Bool

    var body: some View {
        VStack(spacing: 4) {
            AuthenticatedImage(client: client, url: url, width: 140, height: 140)
                .cornerRadius(6)
                .overlay(
                    RoundedRectangle(cornerRadius: 6)
                        .stroke(isSelected ? Color.accentColor : Color.clear, lineWidth: 3)
                )
                .shadow(color: isSelected ? Color.accentColor.opacity(0.3) : Color.black.opacity(0.1),
                        radius: isSelected ? 4 : 2)

            Text(photo.filename)
                .font(.caption2)
                .lineLimit(1)
                .truncationMode(.middle)
                .foregroundColor(isSelected ? .accentColor : .secondary)
        }
        .padding(4)
        .background(isSelected ? Color.accentColor.opacity(0.1) : Color.clear)
        .cornerRadius(8)
    }
}
