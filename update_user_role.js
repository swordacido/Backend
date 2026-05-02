// Script para actualizar el rol y nombre de usuario en MongoDB
// Ejecutar con: mongosh "mongodb://localhost:27017/huasteca" update_user_role.js

db.users.updateOne(
  { email: "abigailcastanedavazquez@gmail.com" },
  {
    $set: {
      name: "umpalumpa",
      role: "VENDEDOR",
      updated_at: new Date()
    }
  }
);

print("Usuario actualizado:");
printjson(db.users.findOne({ email: "abigailcastanedavazquez@gmail.com" }));
