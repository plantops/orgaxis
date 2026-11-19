# Knowledge Base: Thiết kế Hệ thống Onboarding & Xác thực 2FA cho Nhân sự Đa Đối Tượng (Có & Không Email / SĐT)

**Đối tượng áp dụng:** Doanh nghiệp có nhiều nhóm nhân sự (~100+ người): khối văn phòng (có email), khối vận hành/nhà máy/kho bãi (không có email công ty, không dùng SĐT cá nhân để đăng ký), và nhân sự lớn tuổi/không rành công nghệ.  
**Mục tiêu:** Đơn giản hóa tối đa bước kích hoạt lần đầu (Onboarding), đảm bảo bảo mật chuẩn doanh nghiệp (2FA / Audit Log), chi phí duy trì 0đ và vận hành linh hoạt trên mọi thiết bị.

---

## 1. Chiến lược Hybrid Onboarding (Phân luồng thông minh)

Hệ thống tự động phân luồng khởi tạo tài khoản dựa trên dữ liệu đầu vào khi Admin import danh sách nhân viên (`Mã NV`, `Họ tên`, `Ngày sinh`, `Email` [tùy chọn]):

| Nhóm nhân sự | Điều kiện dữ liệu | `username` | Mật khẩu lần đầu (`1st pass`) | Kênh nhận thông tin |
| :--- | :--- | :--- | :--- | :--- |
| **Nhóm A (Có Email)** | Cột `Email` có dữ liệu | Mã NV (vd: `V0134`) | Chuỗi ngẫu nhiên 8 ký tự (vd: `K9mP2xLq`) | Tự động gửi qua Email kèm link kích hoạt |
| **Nhóm B (Không Email)** | Cột `Email` để trống | Mã NV (vd: `V0134`) | 4 số ngày/tháng sinh `DDMM` (vd: sinh 15/08 $\rightarrow$ `1508`) | Không cần gửi tin; nhân viên tự nhập theo quy tắc thông báo chung |

### Hàng rào bảo mật bắt buộc cho Nhóm B (Chống đoán mật khẩu `DDMM`)
Vì quy tắc `Mã NV + DDMM` dễ đoán, tài khoản ở trạng thái chờ kích hoạt (`PENDING`) phải được khóa chặt bằng 3 lớp:
1. **Giới hạn cửa sổ kích hoạt (Activation Window):** Mật khẩu `DDMM` chỉ có hiệu lực trong **48 – 72 giờ** kể từ lúc mở đợt Onboarding (hoặc từ lúc Tổ trưởng bấm mở kích hoạt cho nhân viên đó). Quá hạn sẽ tự động khóa (`EXPIRED`).
2. **Chống dò quét (Strict Rate-Limit):** Khóa tài khoản ngay sau **3 lần nhập sai** `DDMM` liên tiếp (ngăn việc thử 366 tổ hợp ngày sinh).
3. **Zero-Access Before Setup:** Đăng nhập bằng `1st pass` chỉ cấp `onboarding_token` tạm thời, chặn mọi truy cập vào dữ liệu nghiệp vụ cho tới khi nhân viên đổi mật khẩu mới (cấm trùng lại `DDMM` hoặc `Mã NV`) và hoàn tất thiết lập xác thực.

### Chuẩn hóa UX đầu vào trên Mobile
* Tự động viết hoa và cắt khoảng trắng `username` (gõ `v0134` tự hiểu là `V0134`).
* Tự động lọc bỏ dấu gạch chéo `/` ở mật khẩu lần đầu (gõ `15/08` hay `1508` đều hợp lệ).

---

## 2. Cơ chế Xác thực Chính: Username + Password + TOTP (2FA Offline)

Thay vì phụ thuộc vào SMS OTP (tốn phí duy trì ~3 triệu/năm, thủ tục đăng ký Brandname 5–10 ngày, phụ thuộc sóng điện thoại), hệ thống sử dụng chuẩn **TOTP (RFC 6238)** hoạt động hoàn toàn offline trên điện thoại nhân viên.

### Lựa chọn ứng dụng 2FA trên điện thoại nhân viên
* **iPhone (iOS 18+):** Sử dụng ứng dụng **Mật khẩu (Apple Passwords)** tích hợp sẵn (tự động điền cả tài khoản lẫn mã 6 số).
* **Android (Tích hợp sẵn):** Sử dụng **Trình quản lý mật khẩu của Google (Google Password Manager)** trong *Cài đặt > Google > Tự động điền*.
* **Chuẩn hóa đa nền tảng (Khuyên dùng cho 100 người):** **Google Authenticator** hoặc **2FAS Auth** để đồng nhất giao diện hỗ trợ trên mọi đời máy Android (kể cả máy nội địa không có dịch vụ Google) và iPhone cũ.

### Thiết kế màn hình cài đặt 2FA khi mở Web App trên chính điện thoại cá nhân
Vì camera điện thoại không thể tự quét mã QR hiển thị trên chính màn hình của máy đó, giao diện cài đặt 2FA cần bố trí từ trên xuống dưới theo thứ tự ưu tiên:
1. **Nút 1-chạm (`otpauth://` URI) — Ưu tiên số 1:**
   Gắn đường dẫn chuẩn vào nút bấm chính để hệ điều hành tự mở ứng dụng xác thực và lưu khóa chỉ với 1 lần bấm:
   ```text
   otpauth://totp/TenCongTy:V0134?secret=JBSWY3DPEHPK3PXP&issuer=TenCongTy
   ```
2. **Nút Sao chép mã khóa (Setup Key) — Dự phòng khi trình duyệt (Zalo/Messenger) chặn chuyển hướng:**
   Hiển thị chuỗi mã bí mật (vd: `JBSW Y3DP EHPK 3PXP`) kèm nút **Sao chép**. Nhân viên mở app xác thực > chọn **Nhập khóa thiết lập (Enter Setup Key)** > dán mã và giữ loại khóa **Dựa trên thời gian (Time-based)**.
3. **Hình ảnh mã QR — Đặt dưới cùng:**
   Dùng khi nhân viên mở web app trên máy tính/màn hình khác và dùng điện thoại để quét.

### Quy tắc Bảo mật & Giảm phiền hà khi vận hành
* **Remember Device (Tin cậy thiết bị 30 ngày):** Cấp cookie `HttpOnly, Secure, SameSite=Strict` chứa `device_token` có hạn 30 ngày sau khi xác thực 2FA. Nhân viên chỉ nhập mã 6 số mỗi tháng 1 lần trên máy quen thuộc, nhưng bị hỏi ngay nếu đăng nhập từ thiết bị lạ.
* **Xử lý lệch giờ (Clock Drift):** Backend chấp nhận sai số `skew = 1` (chu kỳ 30s hiện tại, liền trước và liền sau).
* **Khôi phục khi mất máy:** Cấp **8–10 Backup Codes** dùng 1 lần lúc Onboarding. Nếu mất cả máy lẫn Backup Codes, Tổ trưởng / Admin xác minh mặt tại chỗ và bấm **Reset tài khoản** (ghi log kiểm toán) để đưa tài khoản về lại luồng kích hoạt lần đầu.

---

## 3. Giải pháp cho Nhân sự Không Rành Công Nghệ / Không Dùng Smartphone

Tuyệt đối **không** chia sẻ mật khẩu hoặc cài hộ app 2FA của nhân viên lên điện thoại của người khác. Áp dụng 3 cơ chế thay thế có kiểm soát:

1. **Chế độ Kiosk / Trạm quét chung (Shared Kiosk + Thẻ QR cá nhân):**
   * Đặt 1–2 máy tính bảng tại tổ sản xuất (định danh cứng thiết bị qua Client Certificate / Device Token).
   * Nhân viên đưa **Thẻ nhân viên in mã QR ký số** (`EMP:V0134:HMAC_SIGNATURE`) trước camera để tự mở màn hình thao tác (chấm công, nhận đồ bảo hộ, xem phiếu lương) và tự động đăng xuất sau **10–15 giây**.
2. **Tổ trưởng "Thao tác hộ" có phân quyền (Act-on-Behalf / Proxy Access):**
   * Tổ trưởng đăng nhập bằng **chính tài khoản và 2FA của Tổ trưởng**, chọn chế độ **"Thao tác thay nhân viên"** $\rightarrow$ chọn `V0134`.
   * Giao diện hiển thị cảnh báo rõ ràng đang thao tác thay cho ai. Hệ thống ghi nhận đồng thời cả `subject_user_id` (nhân viên) và `actor_user_id` (Tổ trưởng) vào bảng lịch sử.
3. **Xác nhận tại chỗ bằng Mã PIN 4 số hoặc Chữ ký cảm ứng:**
   * Với các tác vụ nhạy cảm (xác nhận bảng công, tạm ứng lương) do Tổ trưởng nhập hộ, màn hình bật ô để nhân viên tự bấm **Mã PIN 4 số** (đã cài lúc Onboarding) hoặc ký tay trực tiếp lên màn hình của Tổ trưởng để xác nhận.

---

## 4. Thiết kế Database Chuẩn (PostgreSQL)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,          -- Mã nhân viên viết hoa (vd: V0134)
    full_name VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE,                     -- NULL nếu thuộc nhóm không email
    password_hash TEXT NOT NULL,                   -- Hash của 1st pass (Random/DDMM) hoặc pass mới
    pin_hash TEXT,                                 -- Mã PIN 4 số (cho luồng xác nhận tại chỗ/Kiosk)
    totp_secret_enc TEXT,                          -- Secret TOTP mã hóa AES-256-GCM
    totp_enabled BOOLEAN DEFAULT FALSE,
    onboarding_channel VARCHAR(20) NOT NULL,       -- 'EMAIL_RANDOM' hoặc 'DOB_OFFLINE'
    activation_expires_at TIMESTAMPTZ,             -- Hạn dùng của pass lần đầu (vd: +72h)
    must_change_password BOOLEAN DEFAULT TRUE,
    status VARCHAR(20) DEFAULT 'PENDING',          -- PENDING, ACTIVE, EXPIRED, LOCKED
    failed_attempts INT DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE user_backup_codes (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,                       -- Hash của mã dự phòng (dùng 1 lần)
    used_at TIMESTAMPTZ
);

CREATE TABLE activity_logs (
    id BIGSERIAL PRIMARY KEY,
    subject_user_id UUID NOT NULL REFERENCES users(id), -- Nhân viên thụ hưởng nghiệp vụ
    actor_user_id UUID NOT NULL REFERENCES users(id),   -- Người thực sự bấm nút (Tự làm hoặc Tổ trưởng)
    action_type VARCHAR(50) NOT NULL,                   -- VD: 'ONBOARD_COMPLETE', 'CONFIRM_TIMESHEET'
    auth_method VARCHAR(30) NOT NULL,                   -- 'SELF_2FA', 'KIOSK_QR', 'SUPERVISOR_PROXY_PIN'
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 5. Phụ lục: Gửi SMS OTP Nội Bộ Qua Phần Cứng GSM (Tùy chọn mở rộng)

Nếu doanh nghiệp vẫn cần gửi SMS OTP nội bộ (chỉ 1 lần lúc kích hoạt) mà không muốn đăng ký SMS Brandname với nhà mạng, có thể triển khai giải pháp tự host tại chỗ:

* **Kiến trúc hàng đợi bất đồng bộ:** `Web App` $\rightarrow$ `PostgreSQL/Redis Queue` $\rightarrow$ `smstools3 (Linux)` $\rightarrow$ `Modem GSM 4G (Quectel/Wavecom)` hoặc `GSM Gateway (GoIP-1)`.
* **Lưu ý vận hành thực tế:**
  * Bắt buộc dùng thiết bị hỗ trợ **4G LTE** (do Việt Nam đã tắt sóng 2G).
  * Cố định tên cổng USB trên Linux bằng `udev rules` (`/dev/gsm_modem`) và đặt độ trễ `send_delay = 2` giây trong `/etc/smsd.conf`.
  * Gửi tin tiếng Việt **không dấu** (tối đa 160 ký tự/tin), chèn kèm **Mã NV + Thời gian** để biến đổi nội dung tin nhắn, tránh bị nhà mạng khóa SIM vì nghi ngờ spam.