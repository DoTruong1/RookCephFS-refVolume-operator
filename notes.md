# ADD
1. Tạo bằng cách điền vào tên pvc

  - controller sẽ tự lấy thông tin của pv được gắn trên pvc
  - thêm finalizer vào pvc và pv gốc
  - Tạo ra pv mới?
    - người dùng có điền pvcClaimRef vào custom Object hay k? 
      - nếu chưa có thì có tự tạo pvc?
      - nếu có rồi thì thôi ?

```yaml
 claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: cephfs-pvc
    namespace: default
```
# REMOVE
1. Xóa từ cái gốc
- Tìm đến tất cả các pv ref được tạo bởi controller
- Xóa pvc đi kèm
- Xóa pv, pvc gốc
2. Xóa cái con
- Xóa CRDs tương ứng trong namespace của ứng dụng gốc
