using Antlr.Runtime.Tree;
using System;
using System.Collections.Generic;
using System.Data.SqlClient;
using System.Linq;
using System.Web;
using System.Web.UI;
using System.Web.UI.WebControls;

namespace WebApplication6.Account
{
    public partial class Narudzbine : System.Web.UI.Page
    {
        protected void Page_Load(object sender, EventArgs e)
        {

        }

        protected void Button1_Click(object sender, EventArgs e)
        {
            int sifra = int.Parse(DropDownList1.SelectedValue.ToString());
            int kolicina = int.Parse(TextBox2.Text);
            int zeljena_cena=0;

            string cs = "Data Source=DESKTOP-6V270U1\\ILIJA;Initial Catalog=lazar;Integrated Security=True";
            SqlConnection con = new SqlConnection(cs);
            con.Open();
            try {
                
                string upit = null;
                if (TextBox1.Text != null || TextBox1.Text != "")
                { zeljena_cena = int.Parse(TextBox1.Text);
                    upit = "insert into narudzbine values(" + sifra + "," + kolicina + "," + zeljena_cena + ")";
                }
                else {  upit = "insert into narudzbine(Sifra_proizvoda,Kolicina) values(" + sifra + "," + kolicina + ")"; }
                SqlCommand cmd = new SqlCommand(upit,con);
                cmd.ExecuteNonQuery();


            }
            catch(Exception ex) { ErrorLabel1.Text = ex.Message; ErrorLabel1.ForeColor = System.Drawing.Color.Red; }
            finally { con.Close(); ErrorLabel1.Text = "Uspesno";ErrorLabel1.ForeColor = System.Drawing.Color.Green; }
        }
    }
}