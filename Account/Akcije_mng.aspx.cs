using System;
using System.Collections.Generic;
using System.Data.SqlClient;
using System.Linq;
using System.Web;
using System.Web.UI;
using System.Web.UI.WebControls;

namespace WebApplication6.Account
{
    public partial class Akcije_mng : System.Web.UI.Page
    {
        protected void Page_Load(object sender, EventArgs e)
        {

        }

        protected void Button1_Click(object sender, EventArgs e)
        {
            string cs = "Data Source=DESKTOP-6V270U1\\ILIJA;Initial Catalog=lazar;Integrated Security=True";
            SqlConnection con = new SqlConnection(cs);
            con.Open();
            try
            {
                int sifra = int.Parse(DropDownList1.SelectedValue.ToString());
                int cena = int.Parse(TextBox1.Text);
                string upit = "UPDATE akcije SET Akciska_cena = " + cena + "  WHERE Sifra_proizvoda=" + sifra + "";
                SqlCommand cmd = new SqlCommand(upit,con);
                cmd.ExecuteNonQuery();
            }
            catch(Exception ex) { ErrorLabel1.Text = ex.Message;ErrorLabel1.ForeColor = System.Drawing.Color.Red; }
            finally { con.Close(); ErrorLabel1.Text = "Uspesno";ErrorLabel1.ForeColor = System.Drawing.Color.Green; }
        }
    }
}