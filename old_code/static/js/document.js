import { apiJson } from "./api.js"
import { responseToNotificationData } from "./utils.js"

export default function document() {
    return {
        showModalCreate: false,
        title: "",
        files: [],
        dragging: false,
        loading: false,

        toggleModalCreate() {
            this.showModalCreate = !this.showModalCreate
            if (!this.showModalCreate) {
                this.reset()
            }
        },

        handleFiles(event) {
            const files = Array.from(event.target.files)
            if (files.length > 0) {
                this.files = [files[0]]
            }
        },


        handleDrop(event) {
            event.preventDefault()
            event.stopPropagation()

            this.dragging = false

            const files = Array.from(event.dataTransfer.files)
            if (files.length > 0) {
                this.files = [files[0]]

                if (this.$refs.fileInput) {
                    this.$refs.fileInput.value = ""
                }
            }
        },

        removeFile(index) {
            this.files.splice(index, 1)
        },

        reset() {
            this.title = ""
            this.files = []
            this.loading = false
            if (this.$refs.fileInput) {
                this.$refs.fileInput.value = ""
            }
        },

        async submitDocument() {
            if (!this.files || this.files.length === 0) {
                throw new Error("Nenhum arquivo selecionado")
            }

            this.loading = true

            const file = this.files[0]

            // lê arquivo como base64
            const fileBase64 = await new Promise((resolve, reject) => {
                const reader = new FileReader()
                reader.onload = () => {
                    // remove prefixo: data:application/pdf;base64,
                    const base64 = reader.result.split(",")[1]
                    resolve(base64)
                }
                reader.onerror = reject
                reader.readAsDataURL(file)
            })

            const res = await apiJson("/api/document_upload", {
                method: "POST",
                body: {
                    title: this.title,
                    filename: file.name,
                    fileBase64: fileBase64
                }
            });
            Alpine.store("toast").fire(res);

            this.$dispatch("reload-page", { url: "/documents" });
            this.$dispatch("new-notification", responseToNotificationData(res))

            this.toggleModalCreate()
        }
    }
}
